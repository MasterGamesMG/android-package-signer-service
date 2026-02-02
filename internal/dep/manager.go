package dep

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	RenamerUrl = "https://github.com/dvaoru/ApkRenamer/releases/download/1.9.7/ApkRenamer.zip"
	JarName    = "renamer.jar"
)

func EnsureRenamer() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	libDir := filepath.Join(cwd, "lib")
	mainJarPath := filepath.Join(libDir, "ApkRenamer", JarName)

	if _, err := os.Stat(mainJarPath); err == nil {
		return nil
	}

	fmt.Println("Downloading dependency package...")

	if err := os.MkdirAll(libDir, 0755); err != nil {
		return err
	}

	tempDir := filepath.Join(cwd, "temp")
	os.MkdirAll(tempDir, 0755)

	tempZip := filepath.Join(tempDir, "renamer.zip")
	if err := downloadFile(RenamerUrl, tempZip); err != nil {
		return err
	}
	defer os.Remove(tempZip)

	fmt.Println("Extracting...")
	return unzipAll(tempZip, libDir)
}

func downloadFile(url, filepath string) error {
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
	fmt.Printf("\rDownloading... %d MB / %d MB", wc.Current/1024/1024, wc.Total/1024/1024)
	return n, nil
}

func unzipAll(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// Zip Slip protection
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
