package tee

/*

This is a wrapper package just to ease out adding logics that
should apply to all callers of the sealer.

XXX: Currently it is equivalent as calling the library directly,
and provides just syntax sugar.

*/

import (
	"encoding/base64"

	"github.com/edgelesssys/ego/ecrypto"
)

// Seal uses the TEE Product Key to encrypt the plaintext
// The Product key is the one bound to the signer pubkey
func Seal(plaintext []byte) (string, error) {
	return SealWithKey("", plaintext)
}

func Unseal(encryptedText string) ([]byte, error) {
	return UnsealWithKey("", encryptedText)
}

func SealWithKey(key string, plaintext []byte) (string, error) {
	additionalKey := []byte{}
	if key != "" {
		additionalKey = []byte(key)
	}

	res, err := ecrypto.SealWithProductKey(plaintext, additionalKey)
	if err != nil {
		return "", err
	}

	b64 := base64.StdEncoding.EncodeToString(res)
	return b64, nil
}

func UnsealWithKey(key string, encryptedText string) ([]byte, error) {
	additionalKey := []byte{}
	if key != "" {
		additionalKey = []byte(key)
	}

	b64, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return nil, err
	}

	return ecrypto.Unseal(b64, additionalKey)
}
