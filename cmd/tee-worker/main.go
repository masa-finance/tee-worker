package main

import (
	"context"
	"github.com/masa-finance/tee-worker/internal/api"
	"github.com/masa-finance/tee-worker/pkg/tee"
	"github.com/sirupsen/logrus"
)

func main() {
	jc := readConfig()
	listenAddress := listenAddress()

	// Set standalone mode first based on configuration
	standalone := standaloneMode()
	tee.SealStandaloneMode = standalone

	if tee.KeyDistributorPubKey != "" {
		logrus.Info("This instance will allow only ", tee.KeyDistributorPubKey, " to set the sealing keys")
	}

	// Initialize worker ID - this will work even if sealing key loading failed
	// The worker ID is designed to be persistent across restarts
	if err := tee.InitializeWorkerID(dataDir); err != nil {
		logrus.Fatalf("Failed to initialize persistent worker ID: %v. Exiting...", err)
	}

	// Set the worker ID in the job configuration
	jc["worker_id"] = tee.WorkerID

	// Start the API
	if err := api.Start(context.Background(), listenAddress, dataDir, standalone, jc); err != nil {
		panic(err)
	}

}
