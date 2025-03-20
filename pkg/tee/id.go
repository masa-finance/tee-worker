package tee

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/edgelesssys/ego/ecrypto"
	"github.com/google/uuid"
)

var (
	WorkerID string // Global variable to store the worker ID
)

// GenerateWorkerID generates a new worker ID.
func GenerateWorkerID() string {
	return uuid.New().String()
}

// SaveWorkerID saves the worker ID to a file in the data directory.
// It uses the same encryption mechanism as the sealing key.
func SaveWorkerID(dataDir, workerID string) error {
	// Create the full path
	filePath := filepath.Join(dataDir, "worker_id")

	// Encrypt the worker ID
	var encryptedID []byte
	var err error

	if !SealStandaloneMode {
		// In normal mode, use the SealingKey
		if SealingKey == "" {
			return fmt.Errorf("sealing key not set, cannot save worker ID")
		}

		// Encrypt the worker ID with AES using our sealing key
		encrypted, encErr := EncryptAES(workerID, SealingKey)
		if encErr != nil {
			return encErr
		}
		encryptedID = []byte(encrypted)
	} else {
		// In standalone mode, use the SGX sealing mechanism
		encryptedID, err = ecrypto.SealWithProductKey([]byte(workerID), []byte{})
		if err != nil {
			return err
		}
	}

	// Write to file
	return os.WriteFile(filePath, encryptedID, 0644)
}

// LoadWorkerID loads the worker ID from a file in the data directory.
func LoadWorkerID(dataDir string) (string, error) {
	// Create the full path
	filePath := filepath.Join(dataDir, "worker_id")

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", nil // File doesn't exist, return empty string
	}

	// Read the encrypted worker ID
	encryptedID, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	var workerID string

	if !SealStandaloneMode {
		// In normal mode, use the SealingKey
		if SealingKey == "" {
			return "", fmt.Errorf("sealing key not set, cannot load worker ID")
		}

		// Decrypt the worker ID with AES using our sealing key
		decrypted, decErr := DecryptAES(string(encryptedID), SealingKey)
		if decErr != nil {
			return "", decErr
		}
		workerID = decrypted
	} else {
		// In standalone mode, use the SGX unsealing mechanism
		rawID, err := ecrypto.Unseal(encryptedID, []byte{})
		if err != nil {
			return "", err
		}
		workerID = string(rawID)
	}

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
		newID := GenerateWorkerID()
		if err := SaveWorkerID(dataDir, newID); err != nil {
			return fmt.Errorf("error saving worker ID: %w", err)
		}
		WorkerID = newID
	} else {
		WorkerID = existingID
	}

	return nil
}
