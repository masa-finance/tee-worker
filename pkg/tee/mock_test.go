package tee

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// mockSeal provides a test implementation of sealing data
func mockSeal(data []byte) ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
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
	// For testing, just return the input data
	// In real implementation, this would decrypt the data
	return data, nil
}

// mockSaveLegacyKey mocks saving a legacy key
func mockSaveLegacyKey(datadir, key string) error {
	sealed, err := mockSeal([]byte(key))
	if err != nil {
		return fmt.Errorf("failed to seal legacy key: %w", err)
	}
	return saveLegacyKey(datadir, string(sealed))
}

// mockLoadLegacyKey mocks loading a legacy key
func mockLoadLegacyKey(datadir string) (string, error) {
	key, err := loadLegacyKey(datadir)
	if err != nil {
		return "", err
	}
	unsealed, err := mockUnseal([]byte(key))
	if err != nil {
		return "", err
	}
	return string(unsealed), nil
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
