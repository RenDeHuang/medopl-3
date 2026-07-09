package routes

import (
	"net/http"

	"opl-cloud/services/control-plane/internal/handler"
)

func RegisterCore(mux *http.ServeMux, h handler.CoreHandler) {
	if h.Register != nil {
		h.Register(mux)
	}
}
