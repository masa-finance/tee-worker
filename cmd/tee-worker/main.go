package main

import (
	"context"
	"fmt"

	"github.com/masa-finance/tee-worker/internal/api"
	"github.com/masa-finance/tee-worker/pkg/tee"
)

func main() {
	jc := readConfig()
	listenAddress := listenAddress()

	if tee.KeyDistributorPubKey != "" {
		fmt.Println("This instance will allow only ", tee.KeyDistributorPubKey, " to set the sealing keys")
	}

	if err := api.Start(context.Background(), listenAddress, dataDir, standaloneMode(), jc); err != nil {
		panic(err)
	}
}
