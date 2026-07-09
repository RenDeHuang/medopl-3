package routes

import (
	"net/http"

	"opl-cloud/services/control-plane/internal/handler"
)

func RegisterResource(mux *http.ServeMux, h handler.ResourceHandler) {
	if h.Register != nil {
		h.Register(mux)
	}
}
