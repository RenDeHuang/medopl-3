package handler

import "net/http"

// AdminHandler owns the HTTP bindings for the admin business domain.
type AdminHandler struct {
	Register func(*http.ServeMux)
}
