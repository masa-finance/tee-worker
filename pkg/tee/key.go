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

// SetKey sets a new key, verifying the signature and adding it to the key ring.
func SetKey(datadir, key, signature string) error {
	// Check if key distributor public key is available
	if KeyDistributorPubKey == "" {
		return fmt.Errorf("failed to decode key distributor public key: no key provided")
	}

	// Verify the signature
	dkey, err := base64.StdEncoding.DecodeString(KeyDistributorPubKey)
	if err != nil {
		return fmt.Errorf("failed to decode key distributor public key: %w", err)
	}

	if err := VerifySignature([]byte(key), []byte(signature), dkey); err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	// Validate key length - must be exactly 32 bytes for AES-256
	if len(key) != 32 {
		return fmt.Errorf("invalid key length: got %d bytes, expected 32 bytes for AES-256 encryption", len(key))
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
	added := CurrentKeyRing.Add(key)
	if added {
		logrus.Info("Added new key to key ring")
	} else {
		logrus.Info("Key already present in key ring, not adding again")
	}

	// Save the key ring
	if err := CurrentKeyRing.Save(datadir); err != nil {
		return fmt.Errorf("failed to save key ring: %w", err)
	}

	return nil
}
