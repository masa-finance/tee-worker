package tee

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

var (
	testKey = []byte("0123456789abcdef0123456789abcdef") // 32 bytes for AES-256
)

// getTestKey returns the key to use for mock encryption/decryption
func getTestKey() []byte {
	if CurrentKeyRing != nil && len(CurrentKeyRing.Keys) > 0 {
		return []byte(CurrentKeyRing.MostRecentKey())
	}
	return testKey
}

// mockSeal provides a test implementation of sealing data
func mockSeal(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(getTestKey())
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

// mockUnseal provides a test implementation of unsealing data
func mockUnseal(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(getTestKey())
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// mockSaveKeyRingEntry mocks saving a key to the keyring
func mockSaveKeyRingEntry(datadir, key string) error {
	// Create a keyring if none exists
	keyRing := CurrentKeyRing
	if keyRing == nil {
		keyRing = NewKeyRing()
	}
	
	// Add the key
	keyRing.Add(key)
	
	// Save the keyring
	return keyRing.Save(datadir)
}

// mockVerifySignature mocks signature verification for testing
func mockVerifySignature(payload []byte, signature []byte, publicKeyBytes []byte) error {
	// For testing, accept any signature that's not empty
	if len(signature) == 0 {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

// mockGenerateSignature mocks signature generation for testing
func mockGenerateSignature(payload []byte, privateKeyBytes []byte) ([]byte, error) {
	// For testing, just return a dummy signature
	return []byte("test-signature"), nil
}
