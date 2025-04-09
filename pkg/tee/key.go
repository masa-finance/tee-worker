package tee

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

var (
	KeyDistributorPubKey string
	CurrentKeyRing       *KeyRing
)

// LoadKey loads the key ring from the data directory.
func LoadKey(datadir string) error {
	logrus.Debug("Loading key ring")

	// Check if directory exists
	if _, err := os.Stat(datadir); os.IsNotExist(err) {
		err := fmt.Errorf("directory does not exist: %s", datadir)
		logrus.Warn(err)
		return err
	}

	// Load the key ring
	var err error
	CurrentKeyRing, err = LoadKeyRing(datadir)
	if err != nil {
		logrus.Warnf("Failed to load key ring: %v", err)
		return err
	}

	// If the key ring is empty, log warning
	if len(CurrentKeyRing.Keys) == 0 {
		logrus.Warn("Loaded key ring is empty, no keys available")
	}

	return nil
}

// SetKeyBytes sets a new binary key, verifying the signature and adding it to the key ring.
// The key must be exactly 32 bytes long for AES-256 encryption.
func SetKeyBytes(datadir string, keyBytes []byte, signatureBytes []byte) error {
	// Check if key distributor public key is available
	if KeyDistributorPubKey == "" {
		return fmt.Errorf("failed to decode key distributor public key: no key provided")
	}

	// Verify the signature
	dkey, err := base64.StdEncoding.DecodeString(KeyDistributorPubKey)
	if err != nil {
		return fmt.Errorf("failed to decode key distributor public key: %w", err)
	}

	if err := VerifySignature(keyBytes, signatureBytes, dkey); err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	// Validate key length - must be exactly 32 bytes for AES-256
	if len(keyBytes) != 32 {
		return fmt.Errorf("invalid key length: got %d bytes, expected 32 bytes for AES-256 encryption", len(keyBytes))
	}

	// Initialize or load the key ring if needed
	if CurrentKeyRing == nil {
		CurrentKeyRing, err = LoadKeyRing(datadir)
		if err != nil {
			logrus.Warnf("Failed to load key ring: %v. Creating new one.", err)
			CurrentKeyRing = NewKeyRing()
		}
	}

	// Add the key to the ring
	added := CurrentKeyRing.AddBytes(keyBytes)

	// Save the key ring to persist the change
	if added {
		if err := CurrentKeyRing.Save(datadir); err != nil {
			return fmt.Errorf("failed to save key ring: %w", err)
		}
	}

	return nil
}

// SetKey sets a new key, verifying the signature and adding it to the key ring.
// This is a convenience wrapper around SetKeyBytes that accepts string parameters.
func SetKey(datadir, key, signature string) error {
	// Delegate to SetKeyBytes which handles all the logic
	return SetKeyBytes(datadir, []byte(key), []byte(signature))
}
