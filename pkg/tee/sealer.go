package tee

/*

This is a wrapper package just to ease out adding logics that
should apply to all callers of the sealer.

*/

import (
	"encoding/base64"
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

func SealWithKey(key string, plaintext []byte) (string, error) {
	additionalKey := []byte{}
	if key != "" {
		additionalKey = []byte(key)
	}

	if SealingKey == "" && !SealStandaloneMode {
		return "", fmt.Errorf("sealing key not set")
	}

	var res string
	var err error
	if !SealStandaloneMode {
		res, err = EncryptAES(string(plaintext), fmt.Sprintf("%s-%s", SealingKey, additionalKey))
	} else {
		resBytes, errSeal := ecrypto.SealWithProductKey(plaintext, additionalKey)
		if errSeal != nil {
			return "", errSeal
		}
		res = string(resBytes)
	}

	b64 := base64.StdEncoding.EncodeToString([]byte(res))
	return b64, err
}

func UnsealWithKey(key string, encryptedText string) ([]byte, error) {
	if SealingKey == "" && !SealStandaloneMode {
		return []byte{}, fmt.Errorf("sealing key not set")
	}

	additionalKey := []byte{}
	if key != "" {
		additionalKey = []byte(key)
	}

	b64, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return nil, err
	}

	var res string
	if !SealStandaloneMode {
		res, err = DecryptAES(string(b64), fmt.Sprintf("%s-%s", SealingKey, additionalKey))
	} else {
		resString, errUnseal := ecrypto.Unseal(b64, additionalKey)
		if errUnseal != nil {
			return nil, errUnseal
		}
		res = string(resString)
	}

	return []byte(res), err
}
