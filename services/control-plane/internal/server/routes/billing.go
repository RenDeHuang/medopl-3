package routes

import (
	"net/http"

	"opl-cloud/services/control-plane/internal/handler"
)

func RegisterBilling(mux *http.ServeMux, h handler.BillingHandler) {
	if h.Register != nil {
		h.Register(mux)
	}
}
