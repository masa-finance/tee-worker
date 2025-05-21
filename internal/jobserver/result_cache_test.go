package jobserver

import (
	"time"

	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResultCache", func() {
	It("should set and get values", func() {
		cache := NewResultCache(1000, time.Duration(600)*time.Second)
		key := "abc"
		val := types.JobResult{Job: types.Job{UUID: key}, Error: ""}
		cache.Set(key, val)
		got, ok := cache.Get(key)
		Expect(ok).To(BeTrue())
		Expect(got.Job.UUID).To(Equal(key))
	})

	It("should evict oldest when max size is reached", func() {
		cache := NewResultCache(3, time.Duration(600)*time.Second)
		for i := 0; i < 5; i++ {
			key := string(rune('a' + i))
			cache.Set(key, types.JobResult{Job: types.Job{UUID: key}})
		}
		Expect(len(cache.entries)).To(Equal(3))
		_, ok := cache.Get("a")
		Expect(ok).To(BeFalse())
	})

	It("should evict by age", func() {
		cache := NewResultCache(10, time.Duration(1)*time.Second)
		key := "expireme"
		cache.Set(key, types.JobResult{Job: types.Job{UUID: key}})
		time.Sleep(1100 * time.Millisecond)
		_, ok := cache.Get(key)
		Expect(ok).To(BeFalse())
	})

	It("should clean up expired entries periodically", func() {
		cache := NewResultCache(10, time.Duration(1)*time.Second)
		key := "periodic"
		cache.Set(key, types.JobResult{Job: types.Job{UUID: key}})
		time.Sleep(2200 * time.Millisecond)
		_, ok := cache.Get(key)
		Expect(ok).To(BeFalse())
	})
})
