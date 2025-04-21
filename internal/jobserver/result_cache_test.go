package jobserver

import (
	"os"
	"time"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/masa-finance/tee-worker/api/types"
)

var _ = Describe("ResultCache", func() {
	BeforeEach(func() {
		os.Unsetenv("RESULT_CACHE_MAX_SIZE")
		os.Unsetenv("RESULT_CACHE_MAX_AGE_SECONDS")
	})

	It("should set and get values", func() {
		cache := NewResultCacheFromEnv()
		key := "abc"
		val := types.JobResult{Job: types.Job{UUID: key}, Error: ""}
		cache.Set(key, val)
		got, ok := cache.Get(key)
		Expect(ok).To(BeTrue())
		Expect(got.Job.UUID).To(Equal(key))
	})

	It("should evict oldest when max size is reached", func() {
		os.Setenv("RESULT_CACHE_MAX_SIZE", "3")
		cache := NewResultCacheFromEnv()
		for i := 0; i < 5; i++ {
			key := string(rune('a' + i))
			cache.Set(key, types.JobResult{Job: types.Job{UUID: key}})
		}
		Expect(len(cache.entries)).To(Equal(3))
		_, ok := cache.Get("a")
		Expect(ok).To(BeFalse())
	})

	It("should evict by age", func() {
		os.Setenv("RESULT_CACHE_MAX_SIZE", "10")
		os.Setenv("RESULT_CACHE_MAX_AGE_SECONDS", "1")
		cache := NewResultCacheFromEnv()
		key := "expireme"
		cache.Set(key, types.JobResult{Job: types.Job{UUID: key}})
		time.Sleep(1100 * time.Millisecond)
		_, ok := cache.Get(key)
		Expect(ok).To(BeFalse())
	})

	It("should clean up expired entries periodically", func() {
		os.Setenv("RESULT_CACHE_MAX_SIZE", "10")
		os.Setenv("RESULT_CACHE_MAX_AGE_SECONDS", "1")
		cache := NewResultCacheFromEnv()
		key := "periodic"
		cache.Set(key, types.JobResult{Job: types.Job{UUID: key}})
		time.Sleep(2200 * time.Millisecond)
		_, ok := cache.Get(key)
		Expect(ok).To(BeFalse())
	})
})
