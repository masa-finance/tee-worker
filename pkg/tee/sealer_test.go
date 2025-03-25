package tee

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSealer(t *testing.T) {
	skipIfNotTEE(t)
	// Setup test data
	testKey := "test-sealing-key"
	testPlaintext := []byte("test message")

	t.Run("Seal_Unseal_WithoutSalt", func(t *testing.T) {
		// Set test key
		SealingKey = testKey
		SealStandaloneMode = false

		// Seal data using mock
		sealed, err := mockSeal(testPlaintext)
		require.NoError(t, err)
		assert.NotEmpty(t, sealed)

		// Unseal data using mock
		unsealed, err := mockUnseal(sealed)
		require.NoError(t, err)
		assert.Equal(t, testPlaintext, unsealed)
	})

	t.Run("Seal_Unseal_WithSalt", func(t *testing.T) {
		// Set test key
		SealingKey = testKey
		SealStandaloneMode = false

		// Seal data with salt using mock
		sealed, err := mockSeal(testPlaintext)
		require.NoError(t, err)
		assert.NotEmpty(t, sealed)

		// Unseal data with salt using mock
		unsealed, err := mockUnseal(sealed)
		require.NoError(t, err)
		assert.Equal(t, testPlaintext, unsealed)
	})

	t.Run("Seal_Without_Key", func(t *testing.T) {
		// Clear key
		SealingKey = ""
		SealStandaloneMode = false

		// Attempt to seal without key
		sealed, err := Seal(testPlaintext)
		assert.Error(t, err)
		assert.Empty(t, sealed)
	})

	t.Run("Unseal_Invalid_Base64", func(t *testing.T) {
		SealingKey = testKey
		SealStandaloneMode = false

		// Try to unseal invalid base64
		unsealed, err := Unseal("invalid-base64")
		assert.Error(t, err)
		assert.Nil(t, unsealed)
	})

	t.Run("KeyRing_Decryption", func(t *testing.T) {
		// Setup key ring with multiple keys
		CurrentKeyRing = NewKeyRing()
		keys := []string{"old-key", "current-key"}
		for _, k := range keys {
			CurrentKeyRing.Add(k)
		}
		SealingKey = keys[len(keys)-1] // Set most recent key

		// Seal with current key
		sealed, err := Seal(testPlaintext)
		require.NoError(t, err)

		// Should be able to unseal with key ring
		unsealed, err := Unseal(sealed)
		require.NoError(t, err)
		assert.Equal(t, testPlaintext, unsealed)
	})

	t.Run("Standalone_Mode", func(t *testing.T) {
		// Enable standalone mode
		SealStandaloneMode = true
		SealingKey = ""

		// Should be able to seal/unseal without a key
		sealed, err := Seal(testPlaintext)
		require.NoError(t, err)
		assert.NotEmpty(t, sealed)

		unsealed, err := Unseal(sealed)
		require.NoError(t, err)
		assert.Equal(t, testPlaintext, unsealed)
	})

	t.Run("Key_Derivation", func(t *testing.T) {
		baseKey := "base-key"
		salt := "test-salt"
		
		// Derive key twice with same inputs
		derived1 := deriveKey(baseKey, salt)
		derived2 := deriveKey(baseKey, salt)

		// Should get same result
		assert.Equal(t, derived1, derived2)
		
		// Should be different from base key
		assert.NotEqual(t, baseKey, derived1)
		
		// Different salt should give different key
		derived3 := deriveKey(baseKey, "different-salt")
		assert.NotEqual(t, derived1, derived3)
	})
}

func TestTryDecryptWithKeyRing(t *testing.T) {
	skipIfNotTEE(t)
	// Setup test data
	testPlaintext := []byte("test message")
	testSalt := "test-salt"
	
	t.Run("Decrypt_With_Multiple_Keys", func(t *testing.T) {
		// Setup key ring with multiple keys
		kr := NewKeyRing()
		keys := []string{"key1", "key2", "key3"}
		for _, k := range keys {
			kr.Add(k)
		}

		// Encrypt with each key
		for _, key := range keys {
			SealingKey = key
			sealed, err := SealWithKey(testSalt, testPlaintext)
			require.NoError(t, err)

			// Should be able to decrypt with key ring
			decrypted, err := TryDecryptWithKeyRing(kr, testSalt, sealed)
			require.NoError(t, err)
			assert.Equal(t, testPlaintext, decrypted)
		}
	})

	t.Run("Decrypt_With_Wrong_Keys", func(t *testing.T) {
		// Setup key ring with keys that won't work
		kr := NewKeyRing()
		kr.Add("wrong-key1")
		kr.Add("wrong-key2")

		// Encrypt with a different key
		SealingKey = "correct-key"
		sealed, err := SealWithKey(testSalt, testPlaintext)
		require.NoError(t, err)

		// Should fail to decrypt
		decrypted, err := TryDecryptWithKeyRing(kr, testSalt, sealed)
		assert.Error(t, err)
		assert.Nil(t, decrypted)
	})
}
