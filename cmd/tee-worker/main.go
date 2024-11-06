package main

import (
	"context"
	"os"

	"github.com/masa-finance/tee-worker/internal/api"
)

func main() {
	listenAddress := os.Getenv("LISTEN_ADDRESS")
	if listenAddress == "" {
		listenAddress = ":8080"
	}

	api.Start(context.Background(), listenAddress)
}
