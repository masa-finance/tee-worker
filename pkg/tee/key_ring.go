package tee

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	Key        string    `json:"key"`
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

// Add adds a new key to the ring, pushing out the oldest if at capacity
// It returns true if the key was newly added, false if it was already present
func (kr *KeyRing) Add(key string) bool {
	kr.mu.Lock()
	defer kr.mu.Unlock()

	// Check if key already exists to avoid duplicates
	for _, entry := range kr.Keys {
		if entry.Key == key {
			// Key already exists, don't add it again
			return false
		}
	}

	// Create a new entry with the current time
	newEntry := KeyEntry{
		Key:        key,
		InsertedAt: time.Now(),
	}

	// Insert at the beginning (most recent)
	kr.Keys = append([]KeyEntry{newEntry}, kr.Keys...)

	// Trim to capacity if needed
	if len(kr.Keys) > MaxKeysInRing {
		kr.Keys = kr.Keys[:MaxKeysInRing]
	}

	// Set the current key to the most recent one
	if len(kr.Keys) > 0 {
		SealingKey = kr.Keys[0].Key
	}

	return true
}

// GetAllKeys returns all keys in the ring, most recent first
func (kr *KeyRing) GetAllKeys() []string {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	keys := make([]string, len(kr.Keys))
	for i, entry := range kr.Keys {
		keys[i] = entry.Key
	}
	return keys
}

// MostRecentKey returns the most recent key, or empty string if no keys
func (kr *KeyRing) MostRecentKey() string {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	if len(kr.Keys) == 0 {
		return ""
	}
	return kr.Keys[0].Key
}

// LoadKeyRing loads a key ring from disk
func LoadKeyRing(dataDir string) (*KeyRing, error) {
	// Create the file path
	ringPath := filepath.Join(dataDir, keyRingFilename)
	legacyPath := filepath.Join(dataDir, "sealing_key")

	// Check if the key ring file exists
	if _, err := os.Stat(ringPath); os.IsNotExist(err) {
		// If the legacy file exists, migrate it
		if _, err := os.Stat(legacyPath); err == nil {
			// Try loading the legacy key
			key, err := loadLegacyKey(dataDir)
			if err != nil {
				return nil, fmt.Errorf("failed to load legacy key: %w", err)
			}

			// Create a new key ring with the legacy key
			keyRing := NewKeyRing()
			keyRing.Add(key)
			logrus.Info("Migrated legacy key to key ring")

			// Save the new key ring
			if err := SaveKeyRing(dataDir, keyRing); err != nil {
				return nil, fmt.Errorf("failed to save migrated key ring: %w", err)
			}

			return keyRing, nil
		}

		// No key ring or legacy key found
		return NewKeyRing(), nil
	}

	// Read the encrypted key ring
	encryptedData, err := os.ReadFile(ringPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key ring file: %w", err)
	}

	// Unseal the data
	data, err := ecrypto.Unseal(encryptedData, []byte{})
	if err != nil {
		// In standalone mode, try reading as plain text
		if SealStandaloneMode {
			data = encryptedData
		} else {
			return nil, fmt.Errorf("failed to unseal key ring: %w", err)
		}
	}

	// Unmarshal the key ring
	var keyRing KeyRing
	if err := json.Unmarshal(data, &keyRing); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key ring: %w", err)
	}

	// Set the sealing key to the most recent key
	if len(keyRing.Keys) > 0 {
		SealingKey = keyRing.Keys[0].Key
		logrus.Infof("Loaded key ring with %d keys", len(keyRing.Keys))
	} else {
		logrus.Warn("Loaded key ring is empty")
	}

	return &keyRing, nil
}

// SaveKeyRing saves a key ring to disk
func SaveKeyRing(dataDir string, keyRing *KeyRing) error {
	if keyRing == nil {
		return fmt.Errorf("key ring is nil")
	}

	// Create the file path
	ringPath := filepath.Join(dataDir, keyRingFilename)

	// Marshal the key ring
	keyRing.mu.RLock()
	data, err := json.Marshal(keyRing)
	keyRing.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("failed to marshal key ring: %w", err)
	}

	// Seal the data
	var encryptedData []byte
	if SealStandaloneMode {
		// In standalone mode, try to seal with SGX
		encryptedData, err = ecrypto.SealWithProductKey(data, []byte{})
		if err != nil {
			// If sealing fails, store as plain text
			encryptedData = data
		}
	} else {
		// In normal mode, use SGX sealing
		encryptedData, err = ecrypto.SealWithProductKey(data, []byte{})
		if err != nil {
			return fmt.Errorf("failed to seal key ring: %w", err)
		}
	}

	// Write the file
	if err := os.WriteFile(ringPath, encryptedData, 0600); err != nil {
		return fmt.Errorf("failed to write key ring file: %w", err)
	}

	logrus.Info("Saved key ring with ", len(keyRing.Keys), " keys")
	return nil
}

// loadLegacyKey loads a key from the legacy format
func loadLegacyKey(dataDir string) (string, error) {
	legacyPath := filepath.Join(dataDir, "sealing_key")
	encryptedKey, err := os.ReadFile(legacyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read legacy key: %w", err)
	}

	key, err := ecrypto.Unseal(encryptedKey, []byte{})
	if err != nil {
		// In standalone mode, try reading as plain text
		if SealStandaloneMode {
			return string(encryptedKey), nil
		}
		return "", fmt.Errorf("failed to unseal legacy key: %w", err)
	}

	return string(key), nil
}

// TryDecryptWithKeyRing attempts to decrypt with all keys in the ring
func TryDecryptWithKeyRing(keyRing *KeyRing, salt string, encryptedText string) ([]byte, error) {
	if keyRing == nil {
		return nil, fmt.Errorf("key ring is nil")
	}

	// Get all keys from the ring
	keyRing.mu.RLock()
	keys := make([]string, len(keyRing.Keys))
	for i, entry := range keyRing.Keys {
		keys[i] = entry.Key
	}
	keyRing.mu.RUnlock()

	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys in key ring")
	}

	// Try each key, starting with the most recent
	var lastErr error
	for i, key := range keys {
		// Derive the key with salt if needed
		derivedKey := key
		if salt != "" {
			derivedKey = deriveKey(key, salt)
		}

		// Decode the base64 encrypted text
		b64, err := base64.StdEncoding.DecodeString(encryptedText)
		if err != nil {
			lastErr = err
			continue
		}

		var plaintext []byte
		if SealStandaloneMode {
			// In standalone mode, try SGX unsealing
			plaintext, err = ecrypto.Unseal(b64, []byte(salt))
			if err != nil {
				lastErr = err
				continue
			}
		} else {
			// In normal mode, try AES decryption
			plaintextStr, err := DecryptAES(string(b64), derivedKey)
			if err != nil {
				lastErr = err
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

	return nil, fmt.Errorf("failed to decrypt with any key: %v", lastErr)
}
