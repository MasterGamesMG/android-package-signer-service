package main

import (
	"log"
	"time"

	"android-package-signer-service/internal/java"
)

func main() {
	log.Println("Android Package Signer Service")

	javaPath, err := java.EnsureJava()
	if err != nil {
		log.Fatalf("Java init failed: %v", err)
	}

	log.Printf("Ready. Java: %s", javaPath)

	for {
		time.Sleep(1 * time.Hour)
	}
}
