package tee

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mockSaveKeyRing mocks saving a key ring for testing
func mockSaveKeyRing(dataDir string, kr *KeyRing) error {
	data, err := json.Marshal(kr)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, keyRingFilename), data, 0600)
}

// mockLoadKeyRing mocks loading a key ring for testing
func mockLoadKeyRing(dataDir string) (*KeyRing, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, keyRingFilename))
	if err != nil {
		return nil, err
	}
	kr := &KeyRing{}
	if err := json.Unmarshal(data, kr); err != nil {
		return nil, err
	}
	return kr, nil
}

var _ = Describe("KeyRing", func() {
	var (
		tmpDir string
		err    error
	)

	BeforeEach(func() {
		if os.Getenv("OE_SIMULATION") == "1" {
			Skip("Skipping TEE tests")
		}

		tmpDir, err = os.MkdirTemp("", "keyring-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Context("when creating a new key ring", func() {
		It("should create an empty key ring", func() {
			kr := NewKeyRing()
			Expect(kr).NotTo(BeNil())
			Expect(kr.Keys).To(BeEmpty())
		})
	})

	Context("when adding keys", func() {
		It("should add a new key successfully", func() {
			kr := NewKeyRing()
			testKey := "test-key-1"

			added := kr.Add(testKey)
			Expect(added).To(BeTrue())
			// For []byte fields, we need to use []byte in the HaveField matcher
			Expect(kr.Keys).To(ContainElement(HaveField("Key", []byte(testKey))))
			Expect(kr.Keys).To(HaveLen(1))
		})

		It("should not add duplicate keys", func() {
			kr := NewKeyRing()
			testKey := "test-key-1"

			added := kr.Add(testKey)
			Expect(added).To(BeTrue())

			added = kr.Add(testKey)
			Expect(added).To(BeFalse())
			Expect(kr.Keys).To(HaveLen(1))
		})
	})

	Context("when saving and loading key ring with mocks", func() {
		It("should persist and load keys correctly using mocks", func() {
			kr := NewKeyRing()
			testKeys := []string{"key1", "key2", "key3"}

			for _, key := range testKeys {
				kr.Add(key)
			}

			err := mockSaveKeyRing(tmpDir, kr)
			Expect(err).NotTo(HaveOccurred())

			loadedKR, err := mockLoadKeyRing(tmpDir)
			Expect(err).NotTo(HaveOccurred())

			Expect(loadedKR.Keys).To(HaveLen(len(testKeys)))
			for _, key := range testKeys {
				// For []byte fields, we need to use []byte in the HaveField matcher
				Expect(loadedKR.Keys).To(ContainElement(HaveField("Key", []byte(key))))
			}
		})

		It("should handle invalid directory with mocks", func() {
			invalidDir := filepath.Join(tmpDir, "nonexistent")
			_, err := mockLoadKeyRing(invalidDir)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when saving and loading key ring with real implementation", func() {
		// Save original standalone mode and restore after tests
		var originalStandaloneMode bool

		BeforeEach(func() {
			// Save original standalone mode
			originalStandaloneMode = SealStandaloneMode
			// Enable standalone mode for testing
			SealStandaloneMode = true
		})

		AfterEach(func() {
			// Restore original standalone mode
			SealStandaloneMode = originalStandaloneMode
		})

		It("should persist and load keys correctly using actual Save and Load methods", func() {
			kr := NewKeyRing()
			testKeys := []string{
				"0123456789abcdef0123456789abcdef", // 32-byte key
				"abcdef0123456789abcdef0123456789", // 32-byte key
				"fedcba9876543210fedcba9876543210", // 32-byte key
			}

			// Add keys to the ring
			for _, key := range testKeys {
				kr.Add(key)
			}

			// Save the key ring using the actual Save method
			err := kr.Save(tmpDir)
			Expect(err).NotTo(HaveOccurred(), "Failed to save key ring")

			// Load the key ring using the actual LoadKeyRing function
			loadedKR, err := LoadKeyRing(tmpDir)
			Expect(err).NotTo(HaveOccurred(), "Failed to load key ring")
			Expect(loadedKR).NotTo(BeNil(), "Loaded key ring should not be nil")

			// Verify the keys were preserved
			Expect(loadedKR.Keys).To(HaveLen(len(testKeys)), "Should have the same number of keys")
			
			// Check that the loaded key ring has all the original keys
			for _, key := range testKeys {
				Found := false
				for _, entry := range loadedKR.Keys {
					if string(entry.Key) == key {
						Found = true
						break
					}
				}
				Expect(Found).To(BeTrue(), "Key should be found in loaded key ring: "+key)
			}
			
			// Verify that the first (most recent) key is available through LatestKey()
			Expect(loadedKR.LatestKey()).To(Equal(testKeys[len(testKeys)-1]), "Latest key should match the last added key")
		})

		It("should handle invalid directory with real implementation", func() {
			// Test with non-existent directory
			invalidDir := filepath.Join(tmpDir, "nonexistent")
			_, err := LoadKeyRing(invalidDir)
			Expect(err).To(HaveOccurred(), "Loading from non-existent directory should fail")
			Expect(err.Error()).To(ContainSubstring("no such file or directory"), "Error should indicate file not found")
			
			// Also test saving to a non-existent directory but with auto-creation
			nestedDir := filepath.Join(tmpDir, "nested", "dirs")
			kr := NewKeyRing()
			kr.Add("0123456789abcdef0123456789abcdef") // 32-byte key
			
			// Save should create the directory structure
			err = kr.Save(nestedDir)
			Expect(err).NotTo(HaveOccurred(), "Save should create directories as needed")
			
			// Verify directory was created
			fileInfo, err := os.Stat(nestedDir)
			Expect(err).NotTo(HaveOccurred(), "Directory should exist after save")
			Expect(fileInfo.IsDir()).To(BeTrue(), "Created path should be a directory")
			
			// Verify key ring file was created
			keyRingPath := filepath.Join(nestedDir, keyRingFilename)
			fileInfo, err = os.Stat(keyRingPath)
			Expect(err).NotTo(HaveOccurred(), "Key ring file should exist")
			Expect(fileInfo.IsDir()).To(BeFalse(), "Key ring file should be a file, not directory")
			
			// Clean up
			os.RemoveAll(filepath.Join(tmpDir, "nested"))
		})
	})

	Context("when managing multiple keys", func() {
		It("should maintain correct order and timestamps", func() {
			kr := NewKeyRing()
			keys := []string{"key1", "key2", "key3"}

			for _, key := range keys {
				time.Sleep(time.Millisecond)
				kr.Add(key)
			}

			Expect(kr.Keys).To(HaveLen(len(keys)))

			// Verify reverse order (most recent first)
			for i := 0; i < len(keys); i++ {
				// Compare the string representation of the Key bytes with the expected key
				expectedKey := keys[len(keys)-1-i]
				Expect(string(kr.Keys[i].Key)).To(Equal(expectedKey))
				Expect(kr.Keys[i].InsertedAt).NotTo(BeZero())
			}

			// Test most recent key
			Expect(kr.MostRecentKey()).To(Equal(keys[len(keys)-1]))
		})

		It("should handle empty key ring", func() {
			kr := NewKeyRing()
			Expect(kr.MostRecentKey()).To(BeEmpty())
		})
	})
})
