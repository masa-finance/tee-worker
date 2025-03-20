package main

import (
	"context"
	"fmt"

	"github.com/masa-finance/tee-worker/internal/api"
	"github.com/masa-finance/tee-worker/pkg/tee"
	"github.com/sirupsen/logrus"
)

func main() {
	jc := readConfig()
	listenAddress := listenAddress()

	if tee.KeyDistributorPubKey != "" {
		fmt.Println("This instance will allow only ", tee.KeyDistributorPubKey, " to set the sealing keys")
	}

	// Set standalone mode first based on configuration
	standalone := standaloneMode()
	tee.SealStandaloneMode = standalone

	if tee.KeyDistributorPubKey != "" {
		fmt.Println("This instance will allow only ", tee.KeyDistributorPubKey, " to set the sealing keys")
	}

	// Load the sealing key if not in standalone mode
	if !standalone {
		if err := tee.LoadKey(dataDir); err != nil {
			logrus.Warnf("Failed to load sealing key: %v. Proceeding without key.", err)
		}
	}

	// Initialize worker ID - this will work even if sealing key loading failed
	// The worker ID is designed to be persistent across restarts
	if err := tee.InitializeWorkerID(dataDir); err != nil {
		logrus.Warnf("Failed to initialize persistent worker ID: %v. Using a temporary ID for this session.", err)

		// Generate a temporary ID for this session
		tempID := tee.GenerateWorkerID()
		tee.WorkerID = tempID
		jc["worker_id"] = tempID

		logrus.Infof("Worker using temporary ID for this session: %s", tempID)
	} else {
		logrus.Infof("Worker initialized with persistent ID: %s", tee.WorkerID)

		// Add WorkerID to the job configuration so it's available to all jobs
		jc["worker_id"] = tee.WorkerID
	}

	if err := api.Start(context.Background(), listenAddress, dataDir, standalone, jc); err != nil {
		panic(err)
	}

	if err := api.Start(context.Background(), listenAddress, dataDir, standaloneMode(), jc); err != nil {
		panic(err)
	}
}
