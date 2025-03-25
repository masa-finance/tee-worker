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

   // Set the sealing key (usually the most recent key)
   tee.SealingKey = "abcdef0123456789abcdef0123456789"

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

   // Set a key for standalone mode (32 bytes for AES-256)
   tee.SealingKey = "0123456789abcdef0123456789abcdef"

Note: When using AES encryption, keys must be exactly 32 bytes long for AES-256.
*/

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/edgelesssys/ego/ecrypto"
	"github.com/sirupsen/logrus"
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
	// Always use the most recent key for encryption
	key := SealingKey
	if salt != "" {
		key = deriveKey(SealingKey, salt)
	}

	// Check if we have a key to use
	if SealingKey == "" && !SealStandaloneMode {
		return "", fmt.Errorf("sealing key not set")
	}

	var res string
	var err error
	if !SealStandaloneMode {
		res, err = EncryptAES(string(plaintext), key)
	} else {
		resBytes, errSeal := ecrypto.SealWithProductKey(plaintext, []byte(salt))
		if errSeal != nil {
			return "", errSeal
		}
		res = string(resBytes)
	}

	b64 := base64.StdEncoding.EncodeToString([]byte(res))
	return b64, err
}

func UnsealWithKey(salt string, encryptedText string) ([]byte, error) {
	// If we have a key ring, try using all keys in the ring
	if CurrentKeyRing != nil && len(CurrentKeyRing.Keys) > 0 {
		result, err := TryDecryptWithKeyRing(CurrentKeyRing, salt, encryptedText)
		if err == nil {
			return result, nil
		}
		// Log error but continue with legacy approach as fallback
		logrus.Debugf("Key ring decryption failed: %v. Trying with current key.", err)
	}

	// Legacy approach - try with current key only
	if SealingKey == "" && !SealStandaloneMode {
		return []byte{}, fmt.Errorf("sealing key not set")
	}

	key := SealingKey
	if salt != "" {
		key = deriveKey(SealingKey, salt)
	}

	b64, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return nil, err
	}

	var res string
	if !SealStandaloneMode {
		res, err = DecryptAES(string(b64), key)
	} else {
		resString, errUnseal := ecrypto.Unseal(b64, []byte(salt))
		if errUnseal != nil {
			return nil, errUnseal
		}
		res = string(resString)
	}

	return []byte(res), err
}
