package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"

	ledgerhttp "opl-cloud/services/ledger/internal/http"
	"opl-cloud/services/ledger/internal/ledger"
)

func main() {
	addr := os.Getenv("LEDGER_ADDR")
	if addr == "" {
		addr = ":8081"
	}

	databaseURL, err := storeDatabaseURL(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	token, err := internalServiceToken(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	store := ledger.Store(ledger.NewMemoryStore())
	if databaseURL != "" {
		db, err := sql.Open("postgres", databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		postgresStore := ledger.NewPostgresStore(db)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := postgresStore.Install(ctx); err != nil {
			log.Fatal(err)
		}
		store = postgresStore
	}

	server := ledgerhttp.NewServer(store, token)
	log.Printf("ledger listening on %s", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
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

func storeDatabaseURL(getenv func(string) string) (string, error) {
	databaseURL := getenv("DATABASE_URL")
	if getenv("NODE_ENV") == "production" && databaseURL == "" {
		return "", errors.New("DATABASE_URL is required for production Ledger persistence")
	}
	return databaseURL, nil
}
