package tee

import (
	"os"

	"github.com/edgelesssys/ego/ecrypto"
)

var (
	KeyDistributorPubKey string
	SealingKey           string
)

func LoadKey(datadir string) error {
	key, err := os.ReadFile(datadir + "/sealing_key")
	if err != nil {
		return err
	}

	key, err = ecrypto.Unseal(key, []byte{})
	if err != nil {
		return err
	}

	SealingKey = string(key)
	return nil
}

func SetKey(datadir, key, signature string) error {
	// Verify the signature
	if err := VerifySignature([]byte(key), []byte(signature), []byte(KeyDistributorPubKey)); err != nil {
		return err
	}

	SealingKey = key

	res, err := ecrypto.SealWithProductKey([]byte(key), []byte{})
	if err != nil {
		return err
	}
	return os.WriteFile(datadir+"/sealing_key", res, 0644)
}
