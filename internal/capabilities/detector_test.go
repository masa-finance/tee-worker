package capabilities_test

import (
	"context"

	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/capabilities"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// MockJobServer implements JobServerInterface for testing
type MockJobServer struct {
	capabilities map[string][]string
}

func (m *MockJobServer) GetWorkerCapabilities() map[string][]string {
	return m.capabilities
}

var _ = Describe("DetectCapabilities", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("with a JobServer", func() {
		It("gets capabilities directly from the workers", func() {
			jobServer := &MockJobServer{
				capabilities: map[string][]string{
					"worker1": {"web-scraper", "telemetry"},
					"worker2": {"twitter-scraper", "searchbyquery"},
				},
			}
			expected := []string{"web-scraper", "telemetry", "twitter-scraper", "searchbyquery"}
			detected, _ := DetectCapabilities(ctx, types.JobConfiguration{}, jobServer)
			Expect(detected).To(ConsistOf(expected))
		})
	})

	Context("without a JobServer (standalone worker)", func() {
		It("detects basic capabilities by default", func() {
			expected := []string{"web-scraper", "telemetry", "tiktok-transcription"}
			detected, _ := DetectCapabilities(ctx, types.JobConfiguration{}, nil)
			Expect(detected).To(ConsistOf(expected))
		})

		It("detects Twitter capabilities when accounts are provided", func() {
			jc := types.JobConfiguration{"twitter_accounts": []string{"user:pass"}}
			expected := []string{"web-scraper", "telemetry", "tiktok-transcription", "searchbyquery", "getbyid", "getprofilebyid"}
			detected, _ := DetectCapabilities(ctx, jc, nil)
			Expect(detected).To(ConsistOf(expected))
		})

		It("detects Twitter capabilities when API keys are provided", func() {
			jc := types.JobConfiguration{"twitter_api_keys": []string{"key1"}}
			expected := []string{"web-scraper", "telemetry", "tiktok-transcription", "searchbyquery", "getbyid", "getprofilebyid"}
			detected, _ := DetectCapabilities(ctx, jc, nil)
			Expect(detected).To(ConsistOf(expected))
		})

		It("detects LinkedIn capabilities when credentials array is provided", func() {
			jc := types.JobConfiguration{"linkedin_credentials": []interface{}{"cred1"}}
			expected := []string{"web-scraper", "telemetry", "tiktok-transcription", "searchbyquery", "getprofile"}
			detected, _ := DetectCapabilities(ctx, jc, nil)
			Expect(detected).To(ConsistOf(expected))
		})

		It("detects LinkedIn capabilities when individual credentials are provided", func() {
			jc := types.JobConfiguration{
				"linkedin_li_at_cookie": "cookie1",
				"linkedin_csrf_token":   "token1",
				"linkedin_jsessionid":   "session1",
			}
			expected := []string{"web-scraper", "telemetry", "tiktok-transcription", "searchbyquery", "getprofile"}
			detected, _ := DetectCapabilities(ctx, jc, nil)
			Expect(detected).To(ConsistOf(expected))
		})

		It("does not detect LinkedIn with incomplete credentials", func() {
			jc := types.JobConfiguration{"linkedin_li_at_cookie": "cookie1"}
			expected := []string{"web-scraper", "telemetry", "tiktok-transcription"}
			detected, _ := DetectCapabilities(ctx, jc, nil)
			Expect(detected).To(ConsistOf(expected))
		})
	})
})

var _ = Describe("MergeCapabilities", func() {
	It("should handle empty manual and some detected", func() {
		Expect(MergeCapabilities("", []string{"a", "b"})).To(ConsistOf("a", "b"))
	})

	It("should combine manual and detected capabilities", func() {
		Expect(MergeCapabilities("c,d", []string{"a", "b"})).To(ConsistOf("a", "b", "c", "d"))
	})

	It("should handle overlapping capabilities without duplicates", func() {
		Expect(MergeCapabilities("a,c", []string{"a", "b"})).To(ConsistOf("a", "b", "c"))
	})

	It("should trim whitespace from manual capabilities", func() {
		Expect(MergeCapabilities(" a ,b,c ", []string{"d"})).To(ConsistOf("a", "b", "c", "d"))
	})
})
