package tee

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)


var _ = Describe("KeyRing", func() {
	// Note: KeyRing tests don't require TEE functionality
	// so we don't need to skip them

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

		It("should enforce maximum 2 keys limit", func() {
			kr := NewKeyRing()
			
			// Add first key
			added := kr.Add("key1")
			Expect(added).To(BeTrue())
			Expect(kr.Keys).To(HaveLen(1))
			
			// Add second key
			added = kr.Add("key2")
			Expect(added).To(BeTrue())
			Expect(kr.Keys).To(HaveLen(2))
			
			// Add third key - should push out the oldest
			added = kr.Add("key3")
			Expect(added).To(BeTrue())
			Expect(kr.Keys).To(HaveLen(2)) // Should still be 2
			
			// Verify the keys are the two most recent ones
			Expect(string(kr.Keys[0].Key)).To(Equal("key3")) // Most recent
			Expect(string(kr.Keys[1].Key)).To(Equal("key2")) // Second most recent
			
			// key1 should have been removed
			for _, entry := range kr.Keys {
				Expect(string(entry.Key)).NotTo(Equal("key1"))
			}
		})

		It("should enforce maximum 2 keys limit with AddBytes", func() {
			kr := NewKeyRing()
			
			// Add keys as bytes
			added := kr.AddBytes([]byte("key1"))
			Expect(added).To(BeTrue())
			
			added = kr.AddBytes([]byte("key2"))
			Expect(added).To(BeTrue())
			
			added = kr.AddBytes([]byte("key3"))
			Expect(added).To(BeTrue())
			
			// Should only have 2 keys
			Expect(kr.Keys).To(HaveLen(2))
			Expect(kr.Keys[0].Key).To(Equal([]byte("key3")))
			Expect(kr.Keys[1].Key).To(Equal([]byte("key2")))
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

			// Should only have 2 keys due to the limit
			Expect(kr.Keys).To(HaveLen(2))

			// Verify reverse order (most recent first)
			// Should have key3 and key2 (key1 was pushed out)
			Expect(string(kr.Keys[0].Key)).To(Equal("key3"))
			Expect(string(kr.Keys[1].Key)).To(Equal("key2"))
			Expect(kr.Keys[0].InsertedAt).NotTo(BeZero())
			Expect(kr.Keys[1].InsertedAt).NotTo(BeZero())

			// Test most recent key
			Expect(kr.MostRecentKey()).To(Equal(keys[len(keys)-1]))
		})

		It("should handle empty key ring", func() {
			kr := NewKeyRing()
			Expect(kr.MostRecentKey()).To(BeEmpty())
		})
	})

	Context("when validating and pruning", func() {
		It("should prune excess keys when ValidateAndPrune is called", func() {
			kr := NewKeyRing()
			
			// Manually add more than MaxKeysInRing keys to simulate a pre-existing condition
			kr.Keys = []KeyEntry{
				{Key: []byte("key1"), InsertedAt: time.Now()},
				{Key: []byte("key2"), InsertedAt: time.Now()},
				{Key: []byte("key3"), InsertedAt: time.Now()},
				{Key: []byte("key4"), InsertedAt: time.Now()},
			}
			
			// Verify we have 4 keys initially
			Expect(kr.Keys).To(HaveLen(4))
			
			// Call ValidateAndPrune
			pruned := kr.ValidateAndPrune()
			
			// Should have pruned 2 keys (4 - 2 = 2)
			Expect(pruned).To(Equal(2))
			Expect(kr.Keys).To(HaveLen(2))
			
			// Should keep the first 2 keys (most recent)
			Expect(string(kr.Keys[0].Key)).To(Equal("key1"))
			Expect(string(kr.Keys[1].Key)).To(Equal("key2"))
		})

		It("should not prune when keys are within limit", func() {
			kr := NewKeyRing()
			kr.Add("key1")
			kr.Add("key2")
			
			pruned := kr.ValidateAndPrune()
			Expect(pruned).To(Equal(0))
			Expect(kr.Keys).To(HaveLen(2))
		})

		It("should handle empty keyring", func() {
			kr := NewKeyRing()
			pruned := kr.ValidateAndPrune()
			Expect(pruned).To(Equal(0))
			Expect(kr.Keys).To(HaveLen(0))
		})
	})

	Context("when checking size", func() {
		It("should return correct size", func() {
			kr := NewKeyRing()
			Expect(kr.Size()).To(Equal(0))
			
			kr.Add("key1")
			Expect(kr.Size()).To(Equal(1))
			
			kr.Add("key2")
			Expect(kr.Size()).To(Equal(2))
			
			kr.Add("key3")
			Expect(kr.Size()).To(Equal(2)) // Limited to 2
		})
	})
})
