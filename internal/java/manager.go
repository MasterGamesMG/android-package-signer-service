package java

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	linuxX64Url   = "https://api.adoptium.net/v3/binary/latest/17/ga/linux/x64/jre/hotspot/normal/eclipse"
	windowsX64Url = "https://api.adoptium.net/v3/binary/latest/17/ga/windows/x64/jre/hotspot/normal/eclipse"
)

func EnsureJava() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	localJrePath := filepath.Join(cwd, "bin", "jre")

	// Check if already installed
	if path, err := findJavaExec(localJrePath); err == nil {
		return path, nil
	}

	fmt.Println("Downloading Portable JRE...")

	if err := os.MkdirAll(localJrePath, 0755); err != nil {
		return "", err
	}

	var downloadUrl string
	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "amd64" {
			downloadUrl = linuxX64Url
		} else {
			return "", fmt.Errorf("unsupported os/arch: %s/%s", runtime.GOOS, runtime.GOARCH)
		}
	case "windows":
		if runtime.GOARCH == "amd64" {
			downloadUrl = windowsX64Url
		} else {
			return "", fmt.Errorf("unsupported os/arch: %s/%s", runtime.GOOS, runtime.GOARCH)
		}
	default:
		return "java", nil
	}

	tempDir := filepath.Join(cwd, "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", err
	}
	tempFile := filepath.Join(tempDir, "jre_temp.archive")
	if err := downloadFileWithProgress(downloadUrl, tempFile); err != nil {
		return "", err
	}
	defer os.Remove(tempFile)

	fmt.Println("Extracting...")

	if runtime.GOOS == "windows" {
		if err := extractZip(tempFile, localJrePath); err != nil {
			return "", err
		}
	} else {
		if err := extractTarGz(tempFile, localJrePath); err != nil {
			return "", err
		}
	}

	return findJavaExec(localJrePath)
}

func downloadFileWithProgress(url, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status: %s", resp.Status)
	}

	size, _ := strconv.Atoi(resp.Header.Get("Content-Length"))
	counter := &WriteCounter{Total: uint64(size)}
	if _, err = io.Copy(out, io.TeeReader(resp.Body, counter)); err != nil {
		return err
	}
	fmt.Print("\n")
	return nil
}

type WriteCounter struct {
	Total   uint64
	Current uint64
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Current += uint64(n)
	wc.PrintProgress()
	return n, nil
}

func (wc *WriteCounter) PrintProgress() {
	// Simple progress bar
	fmt.Printf("\rDownloading... %d MB / %d MB", wc.Current/1024/1024, wc.Total/1024/1024)
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			io.Copy(outFile, tr)
			outFile.Close()
			os.Chmod(target, os.FileMode(header.Mode))
		}
	}
	return nil
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		os.MkdirAll(filepath.Dir(fpath), os.ModePerm)
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
	}
	return nil
}

func findJavaExec(rootDir string) (string, error) {
	var javaPath string
	target := "java"
	if runtime.GOOS == "windows" {
		target = "java.exe"
	}

	_ = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == target {
			javaPath = path
			return io.EOF
		}
		return nil
	})

	if javaPath != "" {
		return javaPath, nil
	}
	return "", fmt.Errorf("not found")
}
