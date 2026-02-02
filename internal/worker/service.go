package worker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type Service struct {
	javaPath string
	jarPath  string
	sem      chan struct{} // Semaphore for concurrency control
}

func New(javaPath, jarPath string, maxConcurrent int) *Service {
	return &Service{
		javaPath: javaPath,
		jarPath:  jarPath,
		sem:      make(chan struct{}, maxConcurrent),
	}
}

// Options defines the configurable parameters for the renaming process.
type Options struct {
	PackageName string
	AppName     string
	IconPath    string
	DeepRename  bool
}

// ProcessApk executes the signing tool safely.
func (s *Service) ProcessApk(ctx context.Context, inputPath, outputDir string, opts Options) (string, error) {
	cacheKey, err := s.calculateCacheKey(inputPath, opts)
	if err != nil {
		return "", fmt.Errorf("hash failed: %v", err)
	}

	cwd, _ := os.Getwd()
	cacheFile := filepath.Join(cwd, "data", "cache", cacheKey+".apk")
	if _, err := os.Stat(cacheFile); err == nil {
		fmt.Printf("[Worker] Cache HIT: %s\n", cacheKey)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return "", err
		}
		destPath := filepath.Join(outputDir, cacheKey+".apk")
		if err := copyFile(cacheFile, destPath); err != nil {
			return "", err
		}
		return destPath, nil
	}

	fmt.Printf("[Worker] Cache MISS: %s. Processing...\n", cacheKey)

	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	cacheDir := filepath.Dir(cacheFile)
	os.MkdirAll(cacheDir, 0755)

	args := []string{
		"-Xmx256m", "-jar", s.jarPath,
		"-a", inputPath,
		"-o", cacheFile,
	}

	if opts.PackageName != "" {
		args = append(args, "-p", opts.PackageName)
	}
	if opts.AppName != "" {
		args = append(args, "-n", opts.AppName)
	}
	if opts.IconPath != "" {
		args = append(args, "-i", opts.IconPath)
	}
	if opts.DeepRename {
		args = append(args, "-d")
	}

	cmd := exec.CommandContext(ctx, s.javaPath, args...)
	cmd.Dir = filepath.Dir(s.jarPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Add Java to PATH for subprocesses (e.g. apktool.jar)
	javaDir := filepath.Dir(s.javaPath)
	cmd.Env = append(os.Environ(),
		"PATH="+javaDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"JAVA_TOOL_OPTIONS=-Xmx512m",
	)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("execution failed: %v", err)
	}

	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return "", fmt.Errorf("output not generated")
	}

	destPath := filepath.Join(outputDir, cacheKey+".apk")
	if err := copyFile(cacheFile, destPath); err != nil {
		return "", err
	}

	return destPath, nil
}

func (s *Service) calculateCacheKey(inputPath string, opts Options) (string, error) {
	h := sha256.New()

	f, err := os.Open(inputPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	h.Write([]byte(opts.PackageName))
	h.Write([]byte(opts.AppName))
	if opts.DeepRename {
		h.Write([]byte("deep"))
	}

	if opts.IconPath != "" {
		if fi, err := os.Open(opts.IconPath); err == nil {
			defer fi.Close()
			io.Copy(h, fi)
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
