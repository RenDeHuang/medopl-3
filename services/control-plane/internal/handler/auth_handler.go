package handler

import "net/http"

// AuthHandler owns the HTTP bindings for the auth business domain.
type AuthHandler struct {
	Register func(*http.ServeMux)
}
