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

	// Load the sealing key first, so it's available for worker ID operations
	if err := tee.LoadKey(dataDir); err != nil {
		logrus.Warnf("Failed to load sealing key: %v. Proceeding without key.", err)
	}

	// Initialize worker ID
	if err := tee.InitializeWorkerID(dataDir); err != nil {
		logrus.Warnf("Failed to initialize worker ID: %v. Proceeding without worker ID.", err)
	} else {
		logrus.Infof("Worker initialized with ID: %s", tee.WorkerID)

		// Add WorkerID to the job configuration so it's available to all jobs
		jc["worker_id"] = tee.WorkerID
	}

	// Start the API
	if err := api.Start(context.Background(), listenAddress, dataDir, standaloneMode(), jc); err != nil {
		panic(err)
	}
}
