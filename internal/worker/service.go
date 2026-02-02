package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
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
	PackageName string // Required: [-p]
	AppName     string // Optional: [-n] (If empty, original is kept)
	IconPath    string // Optional: [-i]
	DeepRename  bool   // Optional: [-d] Search and replace package in all files
}

// ProcessApk executes the signing tool safely.
func (s *Service) ProcessApk(ctx context.Context, inputPath string, opts Options) (string, error) {
	// Acquire concurrency token
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	cwd, _ := os.Getwd()
	outDir := filepath.Join(cwd, "out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	outFilename := fmt.Sprintf("signed_%d.apk", time.Now().UnixNano())
	outputPath := filepath.Join(outDir, outFilename)

	// Build arguments dynamically based on Options
	args := []string{
		"-Xmx256m", "-jar", s.jarPath,
		"-a", inputPath,
		"-o", outputPath,
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

	// Critical: I set the working directory so the jar finds its relative dependencies
	cmd.Dir = filepath.Dir(s.jarPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("[Worker] Processing: %s -> %s\n", inputPath, opts.PackageName)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("execution failed: %v", err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("output not generated: %s", outputPath)
	}

	return outputPath, nil
}
