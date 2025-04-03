package tee

/*
This package provides functionality for sealing and unsealing data in a TEE environment.

Usage:

1. Basic Sealing and Unsealing:

   // Seal data
   sealed, err := tee.Seal([]byte("sensitive data"))
   if err != nil {
       log.Fatal(err)
   }

   // Unseal data
   unsealed, err := tee.Unseal(sealed)
   if err != nil {
       log.Fatal(err)
   }

2. Using Key Ring for Multiple Keys:

   // Initialize key ring
   keyRing := tee.NewKeyRing()

   // Add keys to the ring (32-byte keys for AES-256)
   keyRing.Add("0123456789abcdef0123456789abcdef")
   keyRing.Add("abcdef0123456789abcdef0123456789")

   // Set as current key ring
   tee.CurrentKeyRing = keyRing

3. Using Salt for Key Derivation:

   // Seal with salt
   sealed, err := tee.SealWithKey("my-salt", []byte("sensitive data"))
   if err != nil {
       log.Fatal(err)
   }

   // Unseal with salt
   unsealed, err := tee.UnsealWithKey("my-salt", sealed)
   if err != nil {
       log.Fatal(err)
   }

4. Standalone Mode (for testing):

   // Enable standalone mode
   tee.SealStandaloneMode = true

   // You still need to initialize the keyring with at least one key
   keyRing := tee.NewKeyRing()
   keyRing.Add("0123456789abcdef0123456789abcdef")
   tee.CurrentKeyRing = keyRing

Note: When using AES encryption, keys must be exactly 32 bytes long for AES-256.
*/

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/edgelesssys/ego/ecrypto"
)

var SealStandaloneMode bool

// Seal uses the TEE Product Key to encrypt the plaintext
// The Product key is the one bound to the signer pubkey
func Seal(plaintext []byte) (string, error) {
	return SealWithKey("", plaintext)
}

func Unseal(encryptedText string) ([]byte, error) {
	return UnsealWithKey("", encryptedText)
}

// deriveKey takes an input key and a salt, then generates a new key of the same length
func deriveKey(inputKey, salt string) string {
	hash := hmac.New(sha256.New, []byte(salt))
	hash.Write([]byte(inputKey))
	hashedKey := hash.Sum(nil)

	hashedHex := hex.EncodeToString(hashedKey)

	// Ensure the derived key has the same length as the input key
	if len(hashedHex) > len(inputKey) {
		return hashedHex[:len(inputKey)]
	}
	return hashedHex
}

func SealWithKey(salt string, plaintext []byte) (string, error) {
	// Check if the keyring is available and has keys
	if CurrentKeyRing == nil || len(CurrentKeyRing.Keys) == 0 {
		if !SealStandaloneMode {
			return "", fmt.Errorf("no keys available in key ring")
		}
	}

	// Get the most recent key from the keyring
	key := ""
	if CurrentKeyRing != nil && len(CurrentKeyRing.Keys) > 0 {
		key = CurrentKeyRing.MostRecentKey()
	}

	// Apply salt if provided
	if salt != "" && key != "" {
		key = deriveKey(key, salt)
	}

	var res string
	var err error

	// Handle standalone mode directly
	if SealStandaloneMode {
		resBytes, errSeal := ecrypto.SealWithProductKey(plaintext, []byte(salt))
		if errSeal != nil {
			return "", errSeal
		}
		res = string(resBytes)
	} else if key == "" {
		return "", fmt.Errorf("no encryption key available")
	} else {
		res, err = EncryptAES(string(plaintext), key)
		if err != nil {
			return "", err
		}
	}

	b64 := base64.StdEncoding.EncodeToString([]byte(res))
	return b64, err
}

func UnsealWithKey(salt string, encryptedText string) ([]byte, error) {
	// Try decryption with the keyring if available
	if CurrentKeyRing != nil && len(CurrentKeyRing.Keys) > 0 {
		// Try to decrypt with the keyring
		result, err := CurrentKeyRing.Decrypt(salt, encryptedText)

		// In non-standalone mode, return whether success or failure
		if !SealStandaloneMode || err == nil {
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt with any key in the ring: %w", err)
			}
			return result, nil
		}
		// In standalone mode with error, continue to try with product key
	} else if !SealStandaloneMode {
		// If we're not in standalone mode and no keyring is available, that's an error
		return nil, fmt.Errorf("no keys available in key ring")
	}

	// At this point, we are in standalone mode and either:
	// 1. The keyring decryption failed, or
	// 2. No keyring is available

	// Decode base64 for product key decryption in standalone mode
	b64, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return nil, err
	}

	// Try with product key in standalone mode
	resString, errUnseal := ecrypto.Unseal(b64, []byte(salt))
	if errUnseal != nil {
		return nil, errUnseal
	}
	return []byte(resString), nil
}
