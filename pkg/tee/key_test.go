package tee

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyManagement(t *testing.T) {
	skipIfNotTEE(t)
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "key-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Test key and signature
	testKey := "test-key-1234"
	testSignature := "invalid-signature" // We'll generate proper signatures in specific tests

	t.Run("SetKey_Without_KeyDistributor", func(t *testing.T) {
		// Clear key distributor
		KeyDistributorPubKey = ""

		// Attempt to set key
		err := SetKey(tmpDir, testKey, testSignature)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode key distributor public key")
	})

	t.Run("SetKey_With_Invalid_Signature", func(t *testing.T) {
		// Set an invalid key distributor public key
		KeyDistributorPubKey = base64.StdEncoding.EncodeToString([]byte("invalid-key"))

		// Attempt to set key
		err := SetKey(tmpDir, testKey, testSignature)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid signature")
	})

	t.Run("LoadKey_NonExistent", func(t *testing.T) {
		nonExistentDir := filepath.Join(tmpDir, "nonexistent")
		err := LoadKey(nonExistentDir)
		assert.Error(t, err)
	})

	t.Run("Legacy_Key_Operations", func(t *testing.T) {
		// Save legacy key
		err := mockSaveLegacyKey(tmpDir, testKey)
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(filepath.Join(tmpDir, "sealing_key"))
		assert.NoError(t, err)

		// Load legacy key into key ring
		err = loadLegacyKeyIntoKeyRing(tmpDir)
		require.NoError(t, err)

		// Verify key ring contains the key
		assert.NotNil(t, CurrentKeyRing)
		found := false
		for _, entry := range CurrentKeyRing.Keys {
			if entry.Key == testKey {
				found = true
				break
			}
		}
		assert.True(t, found, "Key not found in key ring")
		assert.Equal(t, testKey, SealingKey)
	})

	t.Run("LoadKey_Legacy_Fallback", func(t *testing.T) {
		// Clear current key ring
		CurrentKeyRing = nil
		SealingKey = ""

		// Save a legacy key
		err := mockSaveLegacyKey(tmpDir, testKey)
		require.NoError(t, err)

		// Load key should fall back to legacy
		err = LoadKey(tmpDir)
		require.NoError(t, err)

		// Verify key was loaded
		assert.Equal(t, testKey, SealingKey)
		assert.NotNil(t, CurrentKeyRing)
		found := false
		for _, entry := range CurrentKeyRing.Keys {
			if entry.Key == testKey {
				found = true
				break
			}
		}
		assert.True(t, found, "Key not found in key ring")
	})

	t.Run("LoadKey_Empty_KeyRing", func(t *testing.T) {
		// Create empty key ring
		kr := NewKeyRing()
		err := mockSaveKeyRing(tmpDir, kr)
		require.NoError(t, err)

		// Clear current state
		CurrentKeyRing = nil
		SealingKey = ""

		// Load key should fall back to legacy
		err = LoadKey(tmpDir)
		require.NoError(t, err)

		// Verify legacy key was loaded
		assert.Equal(t, testKey, SealingKey)
		assert.NotNil(t, CurrentKeyRing)
		found := false
		for _, entry := range CurrentKeyRing.Keys {
			if entry.Key == testKey {
				found = true
				break
			}
		}
		assert.True(t, found, "Key not found in key ring")
	})
}

// Helper function to create a temporary key pair for testing
func createTestKeyPair(t *testing.T) ([]byte, []byte) {
	privateKey, err := generateTestPrivateKey()
	require.NoError(t, err)

	publicKey := extractPublicKey(t, privateKey)
	return privateKey, publicKey
}

// Helper function to generate a test private key
func generateTestPrivateKey() ([]byte, error) {
	// Implementation depends on the actual key generation method used
	// This is just a placeholder
	return []byte("test-private-key"), nil
}

// Helper function to extract public key from private key
func extractPublicKey(t *testing.T, privateKey []byte) []byte {
	// Implementation depends on the actual key extraction method used
	// This is just a placeholder
	return []byte("test-public-key")
}
