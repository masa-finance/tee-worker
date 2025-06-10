package tee

import (
	"encoding/base64"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Key Management", func() {
	var (
		tmpDir        string
		testKey       string
		testSignature string
		err           error
	)

	BeforeEach(func() {
		if os.Getenv("OE_SIMULATION") == "1" {
			Skip("Skipping TEE tests")
		}

		tmpDir, err = os.MkdirTemp("", "key-test-*")
		Expect(err).NotTo(HaveOccurred())

		testKey = "test-key-1234"
		testSignature = "invalid-signature" // We'll generate proper signatures in specific tests
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Context("when setting keys", func() {
		It("should fail without key distributor", func() {
			// Clear key distributor
			KeyDistributorPubKey = ""

			// Attempt to set key
			err := SetKey(tmpDir, testKey, testSignature)
			Expect(err).To(HaveOccurred())
			// When KeyDistributorPubKey is empty, we now get a clear error about that
			Expect(err.Error()).To(ContainSubstring("failed to decode key distributor public key"))
		})

		It("should fail with invalid signature", func() {
			// Set an invalid key distributor public key
			KeyDistributorPubKey = base64.StdEncoding.EncodeToString([]byte("invalid-key"))

			// Attempt to set key
			err := SetKey(tmpDir, testKey, testSignature)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid signature"))
		})
	})


	Context("when working with the key ring", func() {
		It("should properly initialize the key ring", func() {
			// Create and add a key to the ring
			keyRing := NewKeyRing()
			keyRing.Add(testKey)

			// No longer saving to disk

			// Verify key ring contains the key
			Expect(keyRing).NotTo(BeNil())
			// Use Gomega matcher for cleaner test code
			Expect(keyRing.Keys).To(ContainElement(HaveField("Key", []byte(testKey))), "Key not found in key ring")
			Expect(keyRing.MostRecentKey()).To(Equal(testKey))
		})

	})
})

// Helper functions for testing
var (
	createTestKeyPair = func() ([]byte, []byte) {
		privateKey, err := generateTestPrivateKey()
		Expect(err).NotTo(HaveOccurred())

		publicKey := extractPublicKey(privateKey)
		return privateKey, publicKey
	}

	generateTestPrivateKey = func() ([]byte, error) {
		// Implementation depends on the actual key generation method used
		// This is just a placeholder
		return []byte("test-private-key"), nil
	}

	extractPublicKey = func(privateKey []byte) []byte {
		// Implementation depends on the actual key extraction method used
		// This is just a placeholder
		return []byte("test-public-key")
	}
)
