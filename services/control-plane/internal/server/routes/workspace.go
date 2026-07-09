package routes

import (
	"net/http"

	"opl-cloud/services/control-plane/internal/handler"
)

func RegisterWorkspace(mux *http.ServeMux, h handler.WorkspaceHandler) {
	if h.Register != nil {
		h.Register(mux)
	}
}
