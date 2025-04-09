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

	Context("when saving and loading key ring", func() {
		It("should persist and load keys correctly", func() {
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

		It("should handle invalid directory", func() {
			invalidDir := filepath.Join(tmpDir, "nonexistent")
			_, err := mockLoadKeyRing(invalidDir)
			Expect(err).To(HaveOccurred())
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
