package handler

import "net/http"

// BillingHandler owns the HTTP bindings for the billing business domain.
type BillingHandler struct {
	Register func(*http.ServeMux)
}
