package main

import (
	"context"

	"github.com/masa-finance/tee-worker/internal/api"
)

// main is the entrypoint for the tee-worker. It reads the configuration, starts
// the API and blocks until the context is canceled.
func main() {
	jc := readConfig()
	listenAddress := listenAddress()

	api.Start(context.Background(), listenAddress, jc)
}
