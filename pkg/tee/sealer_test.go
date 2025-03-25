package tee

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sealer", func() {
	var (
		testKey       string
		testPlaintext []byte
	)

	BeforeEach(func() {
		if os.Getenv("OE_SIMULATION") == "1" {
			Skip("Skipping TEE tests")
		}

		testKey = "0123456789abcdef0123456789abcdef" // 32 bytes for AES-256
		testPlaintext = []byte("test message")
	})

	Context("when sealing and unsealing without salt", func() {
		BeforeEach(func() {
			SealingKey = testKey
			SealStandaloneMode = false
		})

		It("should seal and unseal data correctly", func() {
			sealed, err := mockSeal(testPlaintext)
			Expect(err).NotTo(HaveOccurred())
			Expect(sealed).NotTo(BeEmpty())

			unsealed, err := mockUnseal(sealed)
			Expect(err).NotTo(HaveOccurred())
			Expect(unsealed).To(Equal(testPlaintext))
		})
	})

	Context("when sealing and unsealing with salt", func() {
		BeforeEach(func() {
			SealingKey = testKey
			SealStandaloneMode = false
		})

		It("should seal and unseal data correctly", func() {
			sealed, err := mockSeal(testPlaintext)
			Expect(err).NotTo(HaveOccurred())
			Expect(sealed).NotTo(BeEmpty())

			unsealed, err := mockUnseal(sealed)
			Expect(err).NotTo(HaveOccurred())
			Expect(unsealed).To(Equal(testPlaintext))
		})
	})

	Context("when sealing without a key", func() {
		BeforeEach(func() {
			SealingKey = ""
			SealStandaloneMode = false
		})

		It("should fail to seal data", func() {
			sealed, err := Seal(testPlaintext)
			Expect(err).To(HaveOccurred())
			Expect(sealed).To(BeEmpty())
		})
	})

	Context("when unsealing invalid data", func() {
		BeforeEach(func() {
			SealingKey = testKey
			SealStandaloneMode = false
		})

		It("should fail to unseal invalid base64", func() {
			unsealed, err := Unseal("invalid-base64")
			Expect(err).To(HaveOccurred())
			Expect(unsealed).To(BeNil())
		})
	})

	Context("when using key ring for decryption", func() {
		BeforeEach(func() {
			CurrentKeyRing = NewKeyRing()
			keys := []string{
				"0123456789abcdef0123456789abcdef", // old key
				"abcdef0123456789abcdef0123456789", // current key
			}
			for _, k := range keys {
				CurrentKeyRing.Add(k)
			}
			SealingKey = keys[len(keys)-1] // Set most recent key
		})

		It("should seal and unseal with key ring", func() {
			sealed, err := Seal(testPlaintext)
			Expect(err).NotTo(HaveOccurred())

			unsealed, err := Unseal(sealed)
			Expect(err).NotTo(HaveOccurred())
			Expect(unsealed).To(Equal(testPlaintext))
		})
	})

	Context("when in standalone mode", func() {
		BeforeEach(func() {
			// Skip if not in TEE environment
			if os.Getenv("OE_SIMULATION") != "1" {
				Skip("Skipping standalone mode test in non-TEE environment")
			}
			SealStandaloneMode = true
			SealingKey = "0123456789abcdef0123456789abcdef"
		})

		It("should seal and unseal without a key", func() {
			sealed, err := Seal(testPlaintext)
			Expect(err).NotTo(HaveOccurred())
			Expect(sealed).NotTo(BeEmpty())

			unsealed, err := Unseal(sealed)
			Expect(err).NotTo(HaveOccurred())
			Expect(unsealed).To(Equal(testPlaintext))
		})
	})

	Context("when deriving keys", func() {
		var (
			baseKey string
			salt    string
		)

		BeforeEach(func() {
			baseKey = "base-key"
			salt = "test-salt"
		})

		It("should derive consistent keys with same inputs", func() {
			derived1 := deriveKey(baseKey, salt)
			derived2 := deriveKey(baseKey, salt)

			Expect(derived1).To(Equal(derived2))
			Expect(derived1).NotTo(Equal(baseKey))

			derived3 := deriveKey(baseKey, "different-salt")
			Expect(derived1).NotTo(Equal(derived3))
		})
	})
})

var _ = Describe("Key Ring Decryption", func() {
	var (
		testPlaintext []byte
		testSalt      string
	)

	BeforeEach(func() {
		if os.Getenv("OE_SIMULATION") == "1" {
			Skip("Skipping TEE tests")
		}

		testPlaintext = []byte("test message")
		testSalt = "test-salt"
	})

	Context("when decrypting with multiple keys", func() {
		var (
			kr   *KeyRing
			keys []string
		)

		BeforeEach(func() {
			kr = NewKeyRing()
			keys = []string{
				"0123456789abcdef0123456789abcdef", // key1
				"abcdef0123456789abcdef0123456789", // key2
				"456789abcdef0123456789abcdef0123", // key3
			}
			for _, k := range keys {
				kr.Add(k)
			}
		})

		It("should decrypt successfully with each key", func() {
			for _, key := range keys {
				SealingKey = key
				sealed, err := SealWithKey(testSalt, testPlaintext)
				Expect(err).NotTo(HaveOccurred())

				decrypted, err := TryDecryptWithKeyRing(kr, testSalt, sealed)
				Expect(err).NotTo(HaveOccurred())
				Expect(decrypted).To(Equal(testPlaintext))
			}
		})
	})

	Context("when decrypting with wrong keys", func() {
		var kr *KeyRing

		BeforeEach(func() {
			kr = NewKeyRing()
			kr.Add("00000000000000000000000000000000") // wrong key 1
			kr.Add("11111111111111111111111111111111") // wrong key 2
			SealingKey = "22222222222222222222222222222222" // correct key
		})

		It("should fail to decrypt", func() {
			sealed, err := SealWithKey(testSalt, testPlaintext)
			Expect(err).NotTo(HaveOccurred())

			decrypted, err := TryDecryptWithKeyRing(kr, testSalt, sealed)
			Expect(err).To(HaveOccurred())
			Expect(decrypted).To(BeNil())
		})
	})
})
