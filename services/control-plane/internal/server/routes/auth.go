package routes

import (
	"net/http"

	"opl-cloud/services/control-plane/internal/handler"
)

func RegisterAuth(mux *http.ServeMux, h handler.AuthHandler) {
	if h.Register != nil {
		h.Register(mux)
	}
}
