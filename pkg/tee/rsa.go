package tee

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
)

// This is a wrapper package to ease out reading from certs that are generated with openssl.
// The keys are generated with the following commands:
// Private key:
// openssl genrsa -out private.pem 2048
// Public key:
// openssl rsa -in private.pem -outform PEM -pubout -out public.pem

// GenerateSignature generates a signature for the payload using the private key.
func GenerateSignature(payload, privateKeyBytes []byte) ([]byte, error) {
	// Hash the payload (this returns a [32]byte which we'll convert to a []byte later)
	hash := sha256.Sum256(payload)

	// Decode the key into a "block"
	privateBlock, _ := pem.Decode(privateKeyBytes)
	if privateBlock == nil || privateBlock.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	// Parse the private key from the block
	privateKey, err := x509.ParsePKCS8PrivateKey(privateBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %s", err)
	}

	// Check the type of the key
	if _, ok := privateKey.(*rsa.PrivateKey); !ok {
		return nil, fmt.Errorf("key is not an RSA key")
	}

	// Sign the hash with the client's private key using PSS
	signatureRaw, err := rsa.SignPSS(rand.Reader, privateKey.(*rsa.PrivateKey), crypto.SHA256, hash[:], nil)
	if err != nil {
		return nil, fmt.Errorf("failed to sign the payload: %s", err)
	}

	// Encode the signature to base64 for easy transport
	signature := base64.StdEncoding.EncodeToString(signatureRaw)

	return []byte(signature), nil
}

// VerifySignature verifies the signature for the payload using the public key.
func VerifySignature(payload []byte, signature []byte, publicKeyBytes []byte) error {
	// Hash the payload (this returns a [32]byte which we'll convert to a []byte later)
	hash := sha256.Sum256(payload)

	// Decode the public key into a "block"
	publicBlock, _ := pem.Decode(publicKeyBytes)
	if publicBlock == nil || publicBlock.Type != "PUBLIC KEY" {
		return fmt.Errorf("failed to decode PEM block containing public key")
	}

	// Parse the public key
	publicKey, err := x509.ParsePKIXPublicKey(publicBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %s", err)
	}

	// Check the type of the key
	if _, ok := publicKey.(*rsa.PublicKey); !ok {
		return fmt.Errorf("key is not an RSA key")
	}

	// Decode the signature from base64
	signatureDecoded, err := base64.StdEncoding.DecodeString(string(signature))
	if err != nil {
		return fmt.Errorf("failed to decode the signature: %s", err)
	}

	// Verify the signature with the client's public key using PSS
	err = rsa.VerifyPSS(publicKey.(*rsa.PublicKey), crypto.SHA256, hash[:], signatureDecoded, nil)
	if err != nil {
		return fmt.Errorf("failed to verify the signature: %s", err)
	}

	return nil
}
