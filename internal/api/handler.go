package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"android-package-signer-service/internal/worker"
)

// Standard JSON Response structure
type JsonResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func sendJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func sendError(w http.ResponseWriter, code int, msg string) {
	sendJSON(w, code, JsonResponse{Status: "error", Message: msg})
}

// Helper to get workspace paths
func getWorkspace(r *http.Request) (string, string, string, error) {
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		return "", "", "", fmt.Errorf("Missing X-Client-ID header")
	}

	cwd, _ := os.Getwd()
	base := filepath.Join(cwd, "data", clientID)
	in := filepath.Join(base, "in")
	out := filepath.Join(base, "out")

	// Create if not exists
	err := os.MkdirAll(in, 0755)
	if err == nil {
		err = os.MkdirAll(out, 0755)
	}

	return in, out, base, err
}

// POST /upload
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	inDir, _, _, err := getWorkspace(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}

	if err := r.ParseMultipartForm(500 << 20); err != nil {
		sendError(w, http.StatusBadRequest, "File too large")
		return
	}

	var apkFilename string
	apkFile, apkHeader, err := r.FormFile("apk")
	if err == nil {
		defer apkFile.Close()
		apkFilename = filepath.Base(apkHeader.Filename)
		safeApkPath := filepath.Join(inDir, apkFilename)
		outApk, _ := os.Create(safeApkPath)
		defer outApk.Close()
		io.Copy(outApk, apkFile)
	}

	var iconFilename string
	iconFile, iconHeader, err := r.FormFile("icon")
	if err == nil {
		defer iconFile.Close()
		iconFilename = filepath.Base(iconHeader.Filename)
		safeIconPath := filepath.Join(inDir, iconFilename)
		outIcon, _ := os.Create(safeIconPath)
		defer outIcon.Close()
		io.Copy(outIcon, iconFile)
	}

	if apkFilename == "" && iconFilename == "" {
		sendError(w, http.StatusBadRequest, "At least one file (apk or icon) is required")
		return
	}

	sendJSON(w, http.StatusOK, JsonResponse{
		Status: "ok",
		Data: map[string]string{
			"filename":      apkFilename,
			"icon_filename": iconFilename,
		},
	})
}

// POST /templates
func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Admin check could go here (e.g. check Header X-Admin-Key)

	if err := r.ParseMultipartForm(500 << 20); err != nil {
		sendError(w, http.StatusBadRequest, "File too large")
		return
	}

	file, header, err := r.FormFile("apk")
	if err != nil {
		sendError(w, http.StatusBadRequest, "Missing 'apk' file")
		return
	}
	defer file.Close()

	// Get Template ID from form or use filename
	id := r.FormValue("id")
	if id == "" {
		id = filepath.Base(header.Filename)
	}

	// Save to data/templates/
	cwd, _ := os.Getwd()
	templateDir := filepath.Join(cwd, "data", "templates")
	os.MkdirAll(templateDir, 0755)

	targetPath := filepath.Join(templateDir, id)
	out, err := os.Create(targetPath)
	if err != nil {
		sendError(w, http.StatusInternalServerError, "Storage failed")
		return
	}
	defer out.Close()
	io.Copy(out, file)

	sendJSON(w, http.StatusOK, JsonResponse{
		Status: "ok",
		Data:   map[string]string{"template_id": id},
	})
}

// POST /process
type ProcessRequest struct {
	Filename     string `json:"filename"` // Optional (Legacy/Upload flow)
	TemplateID   string `json:"template_id"`
	PackageName  string `json:"package_name"`
	AppName      string `json:"app_name"`
	IconFilename string `json:"icon_filename"`
}

func (s *Server) handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	inDir, outDir, _, err := getWorkspace(r)
	if err != nil {
		sendError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var req ProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Resolve Input Path: Template OR Uploaded File
	var inputPath string
	if req.TemplateID != "" {
		cwd, _ := os.Getwd()
		inputPath = filepath.Join(cwd, "data", "templates", req.TemplateID)
	} else if req.Filename != "" {
		inputPath = filepath.Join(inDir, req.Filename)
	} else {
		sendError(w, http.StatusBadRequest, "Missing template_id or filename")
		return
	}

	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		sendError(w, http.StatusNotFound, "Input file (Template/Upload) not found.")
		return
	}

	// Resolve Icon Path (if provided)
	var iconPath string
	if req.IconFilename != "" {
		iconPath = filepath.Join(inDir, req.IconFilename)
		if _, err := os.Stat(iconPath); os.IsNotExist(err) {
			sendError(w, http.StatusNotFound, "Icon file not found")
			return
		}
	}

	// Prepare options
	opts := worker.Options{
		PackageName: req.PackageName,
		AppName:     req.AppName,
		IconPath:    iconPath,
	}

	// Trigger worker
	outPath, err := s.worker.ProcessApk(r.Context(), inputPath, outDir, opts)
	if err != nil {
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("Processing failed: %v", err))
		return
	}

	// Return download URL
	generatedFile := filepath.Base(outPath)
	downloadUrl := fmt.Sprintf("/download?file=%s", generatedFile)

	sendJSON(w, http.StatusOK, JsonResponse{
		Status: "ok",
		Data:   map[string]string{"download_url": downloadUrl, "file": generatedFile},
	})
}

// GET /download
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, outDir, _, err := getWorkspace(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	filename := r.URL.Query().Get("file")
	if filename == "" || strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	targetPath := filepath.Join(outDir, filename)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	http.ServeFile(w, r, targetPath)
}
