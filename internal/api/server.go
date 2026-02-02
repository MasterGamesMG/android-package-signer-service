package api

import (
	"android-package-signer-service/internal/worker"
	"net/http"
)

type Server struct {
	worker *worker.Service
}

func NewServer(w *worker.Service) *Server {
	return &Server{worker: w}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/upload", s.handleUpload)
	mux.HandleFunc("/process", s.handleProcess)
	mux.HandleFunc("/download", s.handleDownload)
	return mux
}
