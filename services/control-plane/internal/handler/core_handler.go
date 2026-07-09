package handler

import "net/http"

// CoreHandler owns the HTTP bindings for the core business domain.
type CoreHandler struct {
	Register func(*http.ServeMux)
}
