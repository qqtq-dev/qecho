package api

import (
	_ "embed"
	"net/http"
)

//go:embed web/e.html
var participantPage []byte

func (s *Server) participantPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(participantPage)
}
