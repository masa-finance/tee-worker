package tee

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/edgelesssys/ego/ecrypto"
	"github.com/sirupsen/logrus"
)

const (
	// MaxKeysInRing is the number of keys to keep in the ring buffer
	MaxKeysInRing = 3
	// keyRingFilename is the file where the key ring is stored
	keyRingFilename = "sealing_keys.ring"
)

// KeyEntry represents a single key in the key ring with metadata
type KeyEntry struct {
	// Key is stored as []byte to properly handle arbitrary binary data
	Key        []byte    `json:"key"`
	InsertedAt time.Time `json:"inserted_at"`
}

// KeyRing maintains a ring of keys with the most recent at index 0
type KeyRing struct {
	Keys []KeyEntry `json:"keys"`
	mu   sync.RWMutex
}

// NewKeyRing creates a new key ring
func NewKeyRing() *KeyRing {
	return &KeyRing{
		Keys: make([]KeyEntry, 0, MaxKeysInRing),
	}
}

// AddBytes adds a new binary key to the ring, pushing out the oldest if at capacity
// It returns true if the key was newly added, false if it was already present
func (kr *KeyRing) AddBytes(keyBytes []byte) bool {
	kr.mu.Lock()
	defer kr.mu.Unlock()

	// Check if key already exists to avoid duplicates
	for _, entry := range kr.Keys {
		if bytes.Equal(entry.Key, keyBytes) {
			// Key already exists, don't add it again
			return false
		}
	}

	// Create a new entry with the current time
	newEntry := KeyEntry{
		Key:        keyBytes,
		InsertedAt: time.Now(),
	}

	// Insert at the beginning (most recent)
	kr.Keys = append([]KeyEntry{newEntry}, kr.Keys...)

	// Trim to capacity if needed
	if len(kr.Keys) > MaxKeysInRing {
		kr.Keys = kr.Keys[:MaxKeysInRing]
	}

	return true
}

// Add adds a new key to the ring, pushing out the oldest if at capacity
// It returns true if the key was newly added, false if it was already present
// This method provides backward compatibility by converting the string to []byte
func (kr *KeyRing) Add(key string) bool {
	// Convert string key to []byte and delegate to AddBytes
	return kr.AddBytes([]byte(key))
}

// GetAllKeys returns all keys in the ring, most recent first
func (kr *KeyRing) GetAllKeys() []string {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	keys := make([]string, len(kr.Keys))
	for i, entry := range kr.Keys {
		// Convert []byte to string for compatibility
		keys[i] = string(entry.Key)
	}
	return keys
}

// LatestKey returns the most recent key, or empty string if no keys
func (kr *KeyRing) LatestKey() string {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	if len(kr.Keys) == 0 {
		return ""
	}
	// Convert the []byte key to string for backwards compatibility
	return string(kr.Keys[0].Key)
}

// MostRecentKey returns the most recent key, or empty string if no keys
// Deprecated: Use LatestKey instead
func (kr *KeyRing) MostRecentKey() string {
	// For backward compatibility
	return kr.LatestKey()
}

// LoadKeyRing loads a key ring from disk
func LoadKeyRing(dataDir string) (*KeyRing, error) {
	// Disabled to prevent loading sealing keys from disk
	logrus.Debug("Key ring load disabled for security")
	return NewKeyRing(), nil
}

// Save saves the key ring to disk
func (kr *KeyRing) Save(dataDir string) error {
	// Disabled to prevent sealing keys from being persisted to disk
	logrus.Debug("Key ring save disabled for security")
	return nil
}

// loadLegacyKey loads a key from the legacy format
func loadLegacyKey(dataDir string) (string, error) {
	// SECURITY: Disabled to prevent loading sealing keys from disk
	return "", fmt.Errorf("legacy key loading disabled for security")
}

// Decrypt attempts to decrypt with all keys in the ring
// Parameters:
//   - salt: Optional salt for key derivation
//   - encryptedBase64: The encrypted data as a base64-encoded string
//
// Returns:
//   - Decrypted plaintext as bytes
//   - Error if decryption fails with all keys
func (kr *KeyRing) Decrypt(salt string, encryptedBase64 string) ([]byte, error) {
	if kr == nil {
		logrus.Error("key ring is nil")
		return nil, fmt.Errorf("key ring is nil")
	}

	// Get all keys from the ring
	kr.mu.RLock()
	keys := make([]string, len(kr.Keys))
	for i, entry := range kr.Keys {
		// Convert []byte to string for compatibility
		keys[i] = string(entry.Key)
	}
	kr.mu.RUnlock()

	if len(keys) == 0 {
		logrus.Error("no keys in key ring")
		return nil, fmt.Errorf("no keys in key ring")
	}

	// Decode the base64 encrypted text once before trying keys
	// This is more efficient than decoding inside the key loop
	encryptedBytes, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		logrus.Errorf("base64 decode error: %v", err)
		return nil, fmt.Errorf("base64 decode error: %w", err)
	}

	// Try each key, starting with the most recent
	var errors []error
	for i, key := range keys {
		// Derive the key with salt if needed
		derivedKey := key
		if salt != "" {
			derivedKey = deriveKey(key, salt)
		}

		var plaintext []byte
		if SealStandaloneMode {
			// In standalone mode, try SGX unsealing
			// Note: ecrypto.Unseal internally handles salt via its second parameter
			// We pass the salt directly rather than trying to integrate with derivedKey
			// because SGX unsealing has its own salt handling mechanism
			plaintext, err = ecrypto.Unseal(encryptedBytes, []byte(salt))
			if err != nil {
				errors = append(errors, fmt.Errorf("key %d: SGX unseal error: %w", i+1, err))
				continue
			}
		} else {
			// In normal mode, try AES decryption with the derived key
			plaintextStr, err := DecryptAES(string(encryptedBytes), derivedKey)
			if err != nil {
				errors = append(errors, fmt.Errorf("key %d: AES decrypt error: %w", i+1, err))
				continue
			}
			plaintext = []byte(plaintextStr)
		}

		// Decryption successful
		if i > 0 {
			logrus.Infof("Successfully decrypted with key %d from ring", i+1)
		}
		return plaintext, nil
	}

	// Format all collected errors
	if len(errors) > 0 {
		errMsgs := make([]string, len(errors))
		for i, err := range errors {
			errMsgs[i] = err.Error()
		}
		logrus.Errorf("failed to decrypt with any key. Errors: %s", strings.Join(errMsgs, "; "))
		return nil, fmt.Errorf("failed to decrypt with any key. Errors: %s", strings.Join(errMsgs, "; "))
	}
	logrus.Error("failed to decrypt with any key due to unknown error")
	return nil, fmt.Errorf("failed to decrypt with any key due to unknown error")
}
