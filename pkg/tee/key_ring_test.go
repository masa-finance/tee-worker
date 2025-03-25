package tee

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSaveKeyRing mocks saving a key ring for testing
func mockSaveKeyRing(dataDir string, kr *KeyRing) error {
	data, err := json.Marshal(kr)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, keyRingFilename), data, 0600)
}

// mockLoadKeyRing mocks loading a key ring for testing
func mockLoadKeyRing(dataDir string) (*KeyRing, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, keyRingFilename))
	if err != nil {
		return nil, err
	}
	kr := &KeyRing{}
	if err := json.Unmarshal(data, kr); err != nil {
		return nil, err
	}
	return kr, nil
}

func TestKeyRing(t *testing.T) {
	skipIfNotTEE(t)
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "keyring-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Run("NewKeyRing", func(t *testing.T) {
		kr := NewKeyRing()
		assert.NotNil(t, kr)
		assert.Empty(t, kr.Keys)
	})

	t.Run("Add_Key", func(t *testing.T) {
		kr := NewKeyRing()
		testKey := "test-key-1"

		// Add new key
		added := kr.Add(testKey)
		assert.True(t, added)
		for _, entry := range kr.Keys {
			if entry.Key == testKey {
				return
			}
		}
		assert.Fail(t, "Key not found in key ring")
		assert.Equal(t, 1, len(kr.Keys))

		// Try adding same key again
		added = kr.Add(testKey)
		assert.False(t, added)
		assert.Equal(t, 1, len(kr.Keys))
	})

	t.Run("Save_and_Load_KeyRing", func(t *testing.T) {
		kr := NewKeyRing()
		testKeys := []string{"key1", "key2", "key3"}

		// Add multiple keys
		for _, key := range testKeys {
			kr.Add(key)
		}

		// Save key ring
		err := mockSaveKeyRing(tmpDir, kr)
		require.NoError(t, err)

		// Load key ring
		loadedKR, err := mockLoadKeyRing(tmpDir)
		require.NoError(t, err)

		// Verify loaded keys
		assert.Equal(t, len(testKeys), len(loadedKR.Keys))
		for _, key := range testKeys {
			found := false
			for _, entry := range loadedKR.Keys {
				if entry.Key == key {
					found = true
					break
				}
			}
			assert.True(t, found, "Key %s not found in loaded key ring", key)
		}
	})

	t.Run("GetMostRecentKey", func(t *testing.T) {
		kr := NewKeyRing()
		testKeys := []string{"key1", "key2", "key3"}

		// Add keys in sequence
		for _, key := range testKeys {
			kr.Add(key)
		}

		// Most recent key should be the last one added
		assert.Equal(t, testKeys[len(testKeys)-1], kr.MostRecentKey())
	})

	t.Run("Empty_KeyRing", func(t *testing.T) {
		kr := NewKeyRing()
		assert.Empty(t, kr.MostRecentKey())
	})

	t.Run("Multiple_Keys", func(t *testing.T) {
		// Create a new key ring
		kr := NewKeyRing()
		require.NotNil(t, kr)

		// Add multiple keys
		keys := []string{"key1", "key2", "key3"}
		timestamps := make([]time.Time, len(keys))

		// Add keys with different timestamps
		for i, key := range keys {
			// Sleep briefly to ensure different timestamps
			time.Sleep(time.Millisecond)
			timestamps[i] = time.Now()
			kr.Add(key)
		}

		// Verify all keys are present
		require.Equal(t, len(keys), len(kr.Keys))

		// Verify keys are stored in reverse order (most recent first)
		for i := 0; i < len(keys); i++ {
			require.Equal(t, keys[len(keys)-1-i], kr.Keys[i].Key)
			require.False(t, kr.Keys[i].InsertedAt.IsZero())
		}

		// Test getting most recent key
		mostRecent := kr.MostRecentKey()
		require.Equal(t, keys[len(keys)-1], mostRecent)

		// Save and load key ring
		err := mockSaveKeyRing(tmpDir, kr)
		require.NoError(t, err)

		// Create new key ring and load data
		kr2, err := mockLoadKeyRing(tmpDir)
		require.NoError(t, err)

		// Verify all keys are present in loaded key ring in same order
		require.Equal(t, len(keys), len(kr2.Keys))
		for i := 0; i < len(keys); i++ {
			require.Equal(t, kr.Keys[i].Key, kr2.Keys[i].Key)
			require.Equal(t, kr.Keys[i].InsertedAt.Unix(), kr2.Keys[i].InsertedAt.Unix())
		}
	})

	t.Run("Invalid_Directory", func(t *testing.T) {
		invalidDir := filepath.Join(tmpDir, "nonexistent")
		_, err := mockLoadKeyRing(invalidDir)
		assert.Error(t, err)
	})
}
