package handler

import "net/http"

// ResourceHandler owns the HTTP bindings for the resource business domain.
type ResourceHandler struct {
	Register func(*http.ServeMux)
}
