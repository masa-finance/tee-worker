package tee

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)


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
