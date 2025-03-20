package tee

/*

This is a wrapper package just to ease out adding logics that
should apply to all callers of the sealer.

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
