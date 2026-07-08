package main

import (
	"log"
	"net/http"
	"os"

	"opl-cloud/services/fabric/internal/fabric"
	fabrichttp "opl-cloud/services/fabric/internal/http"
)

func main() {
	addr := os.Getenv("FABRIC_ADDR")
	if addr == "" {
		addr = ":8082"
	}

	operationStore := fabric.OperationStore(fabric.NewMemoryOperationStore())
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		store, err := fabric.NewPostgresOperationStore(databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		operationStore = store
	}
	server := fabrichttp.NewServer(fabric.NewServiceWithOperationStore(fabric.NewTencentProvider(), operationStore))
	log.Printf("fabric listening on %s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatal(err)
	}
}
