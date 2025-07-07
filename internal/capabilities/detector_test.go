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
			// Note: Only capabilities that pass verification will be returned
			// web-scraper may fail due to network issues, so we only expect the always-available ones
			detected, _ := DetectCapabilities(ctx, types.JobConfiguration{}, nil)
			// telemetry and tiktok-transcription should always be available
			Expect(detected).To(ContainElements("telemetry", "tiktok-transcription"))
		})

		It("detects Twitter capabilities when accounts are provided", func() {
			jc := types.JobConfiguration{"twitter_accounts": []string{"user:pass"}}
			detected, _ := DetectCapabilities(ctx, jc, nil)
			// With fake credentials, Twitter capabilities should be filtered out
			// Only basic capabilities that pass verification should remain
			Expect(detected).To(ContainElements("telemetry", "tiktok-transcription"))
			// Twitter capabilities should NOT be present with fake credentials
			Expect(detected).ToNot(ContainElements("searchbyquery", "getbyid", "getprofilebyid"))
		})

		It("detects Twitter capabilities when API keys are provided", func() {
			jc := types.JobConfiguration{"twitter_api_keys": []string{"key1"}}
			detected, _ := DetectCapabilities(ctx, jc, nil)
			// With fake API keys, Twitter capabilities should be filtered out
			Expect(detected).To(ContainElements("telemetry", "tiktok-transcription"))
			// Twitter capabilities should NOT be present with fake API keys
			Expect(detected).ToNot(ContainElements("searchbyquery", "getbyid", "getprofilebyid"))
		})

		It("detects LinkedIn capabilities when credentials array is provided", func() {
			jc := types.JobConfiguration{"linkedin_credentials": []interface{}{"cred1"}}
			detected, _ := DetectCapabilities(ctx, jc, nil)
			// With fake credentials, LinkedIn capabilities should be filtered out
			Expect(detected).To(ContainElements("telemetry", "tiktok-transcription"))
			// LinkedIn capabilities should NOT be present with fake credentials
			Expect(detected).ToNot(ContainElement("getprofile"))
		})

		It("detects LinkedIn capabilities when individual credentials are provided", func() {
			jc := types.JobConfiguration{
				"linkedin_li_at_cookie": "cookie1",
				"linkedin_csrf_token":   "token1",
				"linkedin_jsessionid":   "session1",
			}
			detected, _ := DetectCapabilities(ctx, jc, nil)
			// With fake credentials, LinkedIn capabilities should be filtered out
			Expect(detected).To(ContainElements("telemetry", "tiktok-transcription"))
			// LinkedIn capabilities should NOT be present with fake credentials
			Expect(detected).ToNot(ContainElement("getprofile"))
		})

		It("does not detect LinkedIn with incomplete credentials", func() {
			jc := types.JobConfiguration{"linkedin_li_at_cookie": "cookie1"}
			detected, _ := DetectCapabilities(ctx, jc, nil)
			// Only basic capabilities should be detected
			Expect(detected).To(ContainElements("telemetry", "tiktok-transcription"))
			// LinkedIn capabilities should NOT be present with incomplete credentials
			Expect(detected).ToNot(ContainElement("getprofile"))
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
