package main

import (
	"log"
	"time"

	"android-package-signer-service/internal/dep"
	"android-package-signer-service/internal/java"
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

	log.Printf("Ready. Dependency: %s", "ApkRenamer")

	for {
		time.Sleep(1 * time.Hour)
	}
}
