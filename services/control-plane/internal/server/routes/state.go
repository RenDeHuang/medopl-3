package routes

import (
	"net/http"

	"opl-cloud/services/control-plane/internal/handler"
)

func RegisterState(mux *http.ServeMux, h handler.StateHandler) {
	if h.Register != nil {
		h.Register(mux)
	}
}
