package main

import (
	"context"

	"github.com/masa-finance/tee-worker/internal/api"
)

func main() {
	listenAddress := listenAddress()
	jc := readConfig()

	api.Start(context.Background(), listenAddress, jc)
}
