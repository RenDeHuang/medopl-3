package handler

import "net/http"

// WorkspaceHandler owns the HTTP bindings for the workspace business domain.
type WorkspaceHandler struct {
	Register func(*http.ServeMux)
}
