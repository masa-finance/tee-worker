package main

import (
	"context"

	"github.com/masa-finance/tee-worker/internal/api"
)

func main() {
	jc := readConfig()
	listenAddress := listenAddress()

	api.Start(context.Background(), listenAddress, dataDir, jc)
}
