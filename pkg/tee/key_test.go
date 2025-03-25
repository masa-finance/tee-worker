package tee

import (
	"encoding/base64"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Key Management", func() {
	var (
		tmpDir string
		testKey string
		testSignature string
		err error
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
			if os.Getenv("OE_SIMULATION") != "1" {
				Skip("Skipping key distributor test in non-TEE environment")
			}
			// Clear key distributor
			KeyDistributorPubKey = ""

			// Attempt to set key
			err := SetKey(tmpDir, testKey, testSignature)
			Expect(err).To(HaveOccurred())
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

	Context("when loading keys", func() {
		It("should handle non-existent directory", func() {
			nonExistentDir := filepath.Join(tmpDir, "nonexistent")
			err := LoadKey(nonExistentDir)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when handling legacy keys", func() {
		It("should handle legacy key operations", func() {
			if os.Getenv("OE_SIMULATION") != "1" {
				Skip("Skipping legacy key test in non-TEE environment")
			}
			// Save legacy key
			err := mockSaveLegacyKey(tmpDir, testKey)
			Expect(err).NotTo(HaveOccurred())

			// Verify file exists
			_, err = os.Stat(filepath.Join(tmpDir, "sealing_key"))
			Expect(err).NotTo(HaveOccurred())

			// Load legacy key into key ring
			err = loadLegacyKeyIntoKeyRing(tmpDir)
			Expect(err).NotTo(HaveOccurred())

			// Verify key ring contains the key
			Expect(CurrentKeyRing).NotTo(BeNil())
			found := false
			for _, entry := range CurrentKeyRing.Keys {
				if entry.Key == testKey {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "Key not found in key ring")
			Expect(SealingKey).To(Equal(testKey))
		})

		It("should handle legacy key fallback", func() {
			if os.Getenv("OE_SIMULATION") != "1" {
				Skip("Skipping legacy key fallback test in non-TEE environment")
			}
			// Clear current key ring
			CurrentKeyRing = nil
			SealingKey = ""

			// Save a legacy key
			err := mockSaveLegacyKey(tmpDir, testKey)
			Expect(err).NotTo(HaveOccurred())

			// Load key should fall back to legacy
			err = LoadKey(tmpDir)
			Expect(err).NotTo(HaveOccurred())

			// Verify key was loaded
			Expect(SealingKey).To(Equal(testKey))
			Expect(CurrentKeyRing).NotTo(BeNil())
			found := false
			for _, entry := range CurrentKeyRing.Keys {
				if entry.Key == testKey {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "Key not found in key ring")
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
