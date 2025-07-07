package main

import (
	"context"

	"github.com/masa-finance/tee-worker/internal/api"
	"github.com/masa-finance/tee-worker/internal/capabilities"
	"github.com/masa-finance/tee-worker/pkg/tee"
	"github.com/sirupsen/logrus"
)

func main() {
	jc := readConfig()
	listenAddress := listenAddress()
	ctx := context.Background()

	// Set standalone mode first based on configuration
	standalone := standaloneMode()
	tee.SealStandaloneMode = standalone

	// Perform initial capability detection and verification
	// This gives us the list of healthy capabilities and the tracker for the reconciliation loop.
	healthyCapabilities, healthTracker := capabilities.DetectCapabilities(ctx, jc, nil)
	jc["healthy_capabilities"] = healthyCapabilities

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
	if err := api.Start(ctx, listenAddress, dataDir, standalone, jc, healthTracker); err != nil {
		panic(err)
	}
}
