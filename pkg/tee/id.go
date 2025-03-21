package tee

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/edgelesssys/ego/ecrypto"
	"github.com/google/uuid"
)

const (
	WorkerIdKey = "worker_id"
)

var (
	WorkerID string // Global variable to store the worker ID
)

// generateWorkerID generates a new worker ID.
func generateWorkerID() string {
	return uuid.New().String()
}

// saveWorkerID saves the worker ID to a file in the data directory.
// It uses the same encryption mechanism as the sealing key.
func saveWorkerID(dataDir, workerID string) error {
	// Create the full path
	filePath := filepath.Join(dataDir, WorkerIdKey)

	// Encrypt the worker ID
	var encryptedID []byte
	var err error

	encryptedID, err = ecrypto.SealWithProductKey([]byte(workerID), []byte{})
	if err != nil {
		// If SGX sealing fails in standalone mode, store as plain text
		// This is a fallback for environments where SGX is not available
		encryptedID = []byte(workerID)
	}

	// Write to file
	return os.WriteFile(filePath, encryptedID, 0644)
}

// LoadWorkerID loads the worker ID from a file in the data directory.
func LoadWorkerID(dataDir string) (string, error) {
	// Create the full path
	filePath := filepath.Join(dataDir, WorkerIdKey)

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", nil // File doesn't exist, return empty string
	}

	// Read the encrypted worker ID
	encryptedID, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read worker ID: %w", err)
	}

	var workerID string
	rawID, err := ecrypto.Unseal(encryptedID, []byte{})
	if err != nil {
		return "", fmt.Errorf("failed to unseal worker ID: %w", err)
	}
	workerID = string(rawID)

	return workerID, nil
}

// InitializeWorkerID initializes the worker ID.
// If the worker ID doesn't exist, it generates a new one and saves it.
func InitializeWorkerID(dataDir string) error {
	// Try to load the worker ID
	existingID, err := LoadWorkerID(dataDir)
	if err != nil {
		return fmt.Errorf("error loading worker ID: %w", err)
	}

	// If the worker ID doesn't exist, generate a new one and save it
	if existingID == "" {
		newID := generateWorkerID()
		if err := saveWorkerID(dataDir, newID); err != nil {
			return fmt.Errorf("error saving worker ID: %w", err)
		}
		WorkerID = newID
	} else {
		WorkerID = existingID
	}

	return nil
}
