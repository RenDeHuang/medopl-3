package main

import (
	"log"
	"net/http"
	"os"

	"opl-cloud/services/control-plane/internal/clients"
	"opl-cloud/services/control-plane/internal/controlplane"
	controlserver "opl-cloud/services/control-plane/internal/server"
)

func main() {
	addr := os.Getenv("CONTROL_PLANE_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	ledgerURL := os.Getenv("LEDGER_URL")
	fabricURL := os.Getenv("FABRIC_URL")

	service := controlplane.NewService(
		clients.NewLedgerHTTPClient(ledgerURL, nil),
		clients.NewFabricHTTPClient(fabricURL, nil),
	)
	log.Printf("control-plane listening on %s", addr)
	if err := http.ListenAndServe(addr, controlserver.NewServer(service)); err != nil {
		log.Fatal(err)
	}
}
