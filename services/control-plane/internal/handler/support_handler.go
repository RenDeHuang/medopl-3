package handler

import "net/http"

// SupportHandler owns the HTTP bindings for the support business domain.
type SupportHandler struct {
	Register func(*http.ServeMux)
}
