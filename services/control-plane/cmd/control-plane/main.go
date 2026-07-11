package main

import (
	"errors"
	"log"
	"net/http"
	"os"

	"opl-cloud/services/control-plane/internal/clients"
	"opl-cloud/services/control-plane/internal/controlplane"
	controlserver "opl-cloud/services/control-plane/internal/server"
)

func main() {
	addr := controlPlaneAddr()
	ledgerURL := os.Getenv("LEDGER_URL")
	fabricURL := os.Getenv("FABRIC_URL")
	token, err := internalServiceToken(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}

	service := controlplane.NewService(
		clients.NewLedgerHTTPClient(ledgerURL, token, nil),
		clients.NewFabricHTTPClient(fabricURL, token, nil),
	)
	store, err := controlserver.StateStoreFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	handler, err := controlserver.NewPersistentServer(service, store)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("control-plane listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}

func internalServiceToken(getenv func(string) string) (string, error) {
	token := getenv("OPL_INTERNAL_SERVICE_TOKEN")
	if getenv("NODE_ENV") == "production" && token == "" {
		return "", errors.New("OPL_INTERNAL_SERVICE_TOKEN is required in production")
	}
	return token, nil
}

func controlPlaneAddr() string {
	addr := os.Getenv("CONTROL_PLANE_ADDR")
	if addr == "" && os.Getenv("PORT") != "" {
		addr = ":" + os.Getenv("PORT")
	}
	if addr == "" {
		addr = ":8787"
	}
	return addr
}
