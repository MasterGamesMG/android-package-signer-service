# Android Package Signer Service

A robust, self-contained microservice designed to automate Android APK operations (Signing, Renaming, Cloned Builds) in low-resource environments (VPS, Docker, CI/CD).

## Key Features

- **Zero-Dependency Runtime**:
  This service implements a **Self-Managed Portable JRE**. You do NOT need to install Java, JDK, or configure `JAVA_HOME` on your server. The service automatically detects your OS (Windows/Linux) and downloads a lightweight, isolated OpenJDK instance to `./bin/jre` if missing.
  
- **Resources Efficient**:
  Written in Go, the service itself consumes negligible resources (~10MB), ensuring the maximum amount of RAM is available for the underlying Java process to handle APK manipulation.

- **Drop-in Deployment**:
  Just upload the binary and run. Ideal for disposable containers or minimal Linux VPS environments.

## Getting Started

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/MasterGamesMG/android-package-signer-service.git
   cd android-package-signer-service
   ```

2. Build the binary:
   ```bash
   go build -o out/service.exe cmd/server/main.go
   ```
   *(For Linux, use: `env GOOS=linux go build -o out/service-linux cmd/server/main.go`)*

3. Run:
   ```bash
   ./out/service.exe
   ```

### How it Works
1. On startup, the service verifies if a valid Java Runtime exists in `./bin/jre`.
2. If missing, it downloads the correct OpenJDK build for your specific architecture (Windows x64 or Linux x64).
3. Once the environment is ready, it initializes the internal APK processing engine (powered by `ApkRenamer`).

## Legal & Credits

This project acts as an automation wrapper for **ApkRenamer**.
- **ApkRenamer** is developed by [dvaoru](https://github.com/dvaoru).
- Licensed under the **Apache License 2.0**.

This project itself is licensed under **Apache 2.0**.