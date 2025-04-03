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
		It("should handle directory creation and cleanup", func() {
			// First test that a non-existent directory fails
			nonExistentDir := filepath.Join(tmpDir, "nonexistent")
			err := LoadKey(nonExistentDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("directory does not exist"))
			
			// Now create the directory
			err = os.MkdirAll(nonExistentDir, 0755)
			Expect(err).NotTo(HaveOccurred())
			
			// Test that LoadKey now works with the created directory
			err = LoadKey(nonExistentDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(CurrentKeyRing).NotTo(BeNil())
			Expect(CurrentKeyRing.Keys).To(BeEmpty())
			
			// Add a key to test saving works
			testKey := "test-key-for-created-dir"
			CurrentKeyRing.Add(testKey)
			Expect(CurrentKeyRing.MostRecentKey()).To(Equal(testKey))
			
			// Save the keyring
			err = CurrentKeyRing.Save(nonExistentDir)
			Expect(err).NotTo(HaveOccurred())
			
			// Clean up - remove the directory after testing
			err = os.RemoveAll(nonExistentDir)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when working with the key ring", func() {
		It("should properly initialize the key ring", func() {
			// Create and add a key to the ring
			keyRing := NewKeyRing()
			keyRing.Add(testKey)
			
			// Save the key ring
			err := keyRing.Save(tmpDir)
			Expect(err).NotTo(HaveOccurred())

			// Verify key ring contains the key
			Expect(keyRing).NotTo(BeNil())
			found := false
			for _, entry := range keyRing.Keys {
				if entry.Key == testKey {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "Key not found in key ring")
			Expect(keyRing.MostRecentKey()).To(Equal(testKey))
		})

		It("should properly load the key ring", func() {
			// Clear current key ring
			CurrentKeyRing = nil

			// Create and save a key ring
			keyRing := NewKeyRing()
			keyRing.Add(testKey)
			err := keyRing.Save(tmpDir)
			Expect(err).NotTo(HaveOccurred())

			// Load key ring
			err = LoadKey(tmpDir)
			Expect(err).NotTo(HaveOccurred())

			// Verify key ring was loaded
			Expect(CurrentKeyRing).NotTo(BeNil())
			found := false
			for _, entry := range CurrentKeyRing.Keys {
				if entry.Key == testKey {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "Key not found in key ring")
			Expect(CurrentKeyRing.MostRecentKey()).To(Equal(testKey))
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
