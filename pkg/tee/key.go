package tee

import (
	"encoding/base64"
	"fmt"

	"github.com/sirupsen/logrus"
)

var (
	KeyDistributorPubKey string
	CurrentKeyRing       *KeyRing
)


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

	// Initialize the key ring if needed
	if CurrentKeyRing == nil {
		CurrentKeyRing = NewKeyRing()
	}

	// Add the key to the ring
	added := CurrentKeyRing.AddBytes(keyBytes)
	
	if added {
		logrus.Info("Key added to ring (not persisted to disk for security)")
	}

	return nil
}

// SetKey sets a new key, verifying the signature and adding it to the key ring.
// This is a convenience wrapper around SetKeyBytes that accepts string parameters.
func SetKey(datadir, key, signature string) error {
	// Delegate to SetKeyBytes which handles all the logic
	return SetKeyBytes(datadir, []byte(key), []byte(signature))
}
