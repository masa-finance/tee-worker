package tee

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/edgelesssys/ego/ecrypto"
	"github.com/sirupsen/logrus"
)

var (
	KeyDistributorPubKey string
	SealingKey           string
	CurrentKeyRing       *KeyRing
)

// LoadKey loads the sealing key from the data directory.
// This is kept for backward compatibility but will actually load the key ring.
func LoadKey(datadir string) error {
	logrus.Debug("Loading key ring")

	// Load the key ring, which will set SealingKey to the most recent key
	var err error
	CurrentKeyRing, err = LoadKeyRing(datadir)
	if err != nil {
		// If the key ring fails to load, try the legacy method
		logrus.Warnf("Failed to load key ring: %v. Falling back to legacy key.", err)
		return loadLegacyKeyIntoKeyRing(datadir)
	}

	// If the key ring is empty, try loading the legacy key
	if len(CurrentKeyRing.Keys) == 0 {
		logrus.Debug("Key ring is empty, trying to load legacy key")
		if err := loadLegacyKeyIntoKeyRing(datadir); err != nil {
			logrus.Warnf("No keys available: %v", err)
			return err
		}
	}

	return nil
}

// loadLegacyKeyIntoKeyRing loads a legacy key and adds it to the key ring
func loadLegacyKeyIntoKeyRing(datadir string) error {
	// Try to load using the old method
	key, err := loadLegacyKey(datadir)
	if err != nil {
		return fmt.Errorf("failed to load legacy key: %w", err)
	}

	// Initialize the key ring if needed
	if CurrentKeyRing == nil {
		CurrentKeyRing = NewKeyRing()
	}

	// Add the key to the ring
	if added := CurrentKeyRing.Add(key); added {
		logrus.Info("Added legacy key to key ring")
	} else {
		logrus.Debug("Legacy key already present in key ring")
	}

	SealingKey = key
	return nil
}

// SetKey sets a new key, verifying the signature and adding it to the key ring.
func SetKey(datadir, key, signature string) error {
	// Verify the signature
	dkey, err := base64.StdEncoding.DecodeString(KeyDistributorPubKey)
	if err != nil {
		return fmt.Errorf("failed to decode key distributor public key: %w", err)
	}

	if err := VerifySignature([]byte(key), []byte(signature), dkey); err != nil {
		return fmt.Errorf("invalid signature: %w", err)
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
	if err := SaveKeyRing(datadir, CurrentKeyRing); err != nil {
		return fmt.Errorf("failed to save key ring: %w", err)
	}

	// For backward compatibility, also save as legacy key
	if err := saveLegacyKey(datadir, key); err != nil {
		logrus.Warnf("Failed to save legacy key: %v", err)
	}

	SealingKey = key
	return nil
}

// saveLegacyKey saves a key in the legacy format for backward compatibility
func saveLegacyKey(datadir, key string) error {
	res, err := ecrypto.SealWithProductKey([]byte(key), []byte{})
	if err != nil {
		return fmt.Errorf("failed to seal legacy key: %w", err)
	}
	return os.WriteFile(datadir+"/sealing_key", res, 0600)
}
