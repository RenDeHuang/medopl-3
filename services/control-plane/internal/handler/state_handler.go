package handler

import "net/http"

// StateHandler owns the HTTP bindings for the state business domain.
type StateHandler struct {
	Register func(*http.ServeMux)
}
