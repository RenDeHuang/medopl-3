package routes

import (
	"net/http"

	"opl-cloud/services/control-plane/internal/handler"
)

func RegisterAdmin(mux *http.ServeMux, h handler.AdminHandler) {
	if h.Register != nil {
		h.Register(mux)
	}
}
