package main

import (
	"context"

	"github.com/masa-finance/tee-worker/internal/api"
)

func main() {
	jc := readConfig()
	listenAddress := listenAddress()

	if err := api.Start(context.Background(), listenAddress, dataDir, standalone, jc); err != nil {
		panic(err)
	}
}
