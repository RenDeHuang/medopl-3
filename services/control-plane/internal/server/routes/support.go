package routes

import (
	"net/http"

	"opl-cloud/services/control-plane/internal/handler"
)

func RegisterSupport(mux *http.ServeMux, h handler.SupportHandler) {
	if h.Register != nil {
		h.Register(mux)
	}
}
