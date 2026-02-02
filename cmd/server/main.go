package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"android-package-signer-service/internal/api"
	"android-package-signer-service/internal/dep"
	"android-package-signer-service/internal/java"
	"android-package-signer-service/internal/worker"
)

func main() {
	log.Println("Android Package Signer Service")

	// Setup Java
	javaPath, err := java.EnsureJava()
	if err != nil {
		log.Fatalf("Java init failed: %v", err)
	}

	log.Printf("Ready. Java: %s", javaPath)

	// Setup External Dependencies
	if err := dep.EnsureRenamer(); err != nil {
		log.Fatalf("Dependency init failed: %v", err)
	}

	// Initialize Worker
	cwd, _ := os.Getwd()
	jarPath := filepath.Join(cwd, "lib", "ApkRenamer", "renamer.jar")

	workerSvc := worker.New(javaPath, jarPath, 2)

	// Initialize API
	server := api.NewServer(workerSvc)

	// Start HTTP Server
	port := "8080"
	log.Printf("Starting server on :%s", port)

	// Create a Server with timeouts for robustness
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      server.Routes(),
		ReadTimeout:  10 * time.Minute,
		WriteTimeout: 10 * time.Minute,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
