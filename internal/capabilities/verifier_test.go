package capabilities_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/masa-finance/tee-worker/internal/capabilities"
	"github.com/masa-finance/tee-worker/internal/capabilities/health"
)

// mockVerifier is a mock implementation of the Verifier interface for testing.
type mockVerifier struct {
	shouldSucceed bool
	err           error
}

func (m *mockVerifier) Verify(ctx context.Context) (bool, error) {
	if m.shouldSucceed {
		return true, nil
	}
	return false, m.err
}

var _ = Describe("CapabilityVerifier", func() {
	var (
		tracker         *health.Tracker
		verifier        *CapabilityVerifier
		successVerifier *mockVerifier
		failVerifier    *mockVerifier
		ctx             context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		tracker = health.NewTracker()
		verifier = NewCapabilityVerifier(tracker)
		successVerifier = &mockVerifier{shouldSucceed: true}
		failVerifier = &mockVerifier{shouldSucceed: false, err: fmt.Errorf("verification failed")}
	})

	Context("when all capabilities are healthy", func() {
		It("should mark all as healthy in the tracker", func() {
			verifier.RegisterVerifier("cap-a", successVerifier)
			verifier.RegisterVerifier("cap-b", successVerifier)
			verifier.VerifyCapabilities(ctx, []string{"cap-a", "cap-b"})

			statuses := tracker.GetAllStatuses()
			Expect(statuses).To(HaveLen(2))
			Expect(statuses["cap-a"].IsHealthy).To(BeTrue())
			Expect(statuses["cap-b"].IsHealthy).To(BeTrue())
		})
	})

	Context("when one capability fails verification", func() {
		It("should mark the correct capability as unhealthy", func() {
			verifier.RegisterVerifier("cap-a", successVerifier)
			verifier.RegisterVerifier("cap-b", failVerifier)
			verifier.VerifyCapabilities(ctx, []string{"cap-a", "cap-b"})

			statuses := tracker.GetAllStatuses()
			Expect(statuses).To(HaveLen(2))
			Expect(statuses["cap-a"].IsHealthy).To(BeTrue())
			Expect(statuses["cap-b"].IsHealthy).To(BeFalse())
			Expect(statuses["cap-b"].LastError).To(Equal("verification failed"))
		})
	})

	Context("when a capability does not have a registered verifier", func() {
		It("should be assumed healthy", func() {
			verifier.RegisterVerifier("cap-a", successVerifier)
			verifier.VerifyCapabilities(ctx, []string{"cap-a", "unregistered-cap"})

			statuses := tracker.GetAllStatuses()
			Expect(statuses).To(HaveLen(2))
			Expect(statuses["cap-a"].IsHealthy).To(BeTrue())
			Expect(statuses["unregistered-cap"].IsHealthy).To(BeTrue())
			Expect(statuses["unregistered-cap"].LastError).To(BeEmpty())
		})
	})

	Context("when there are no capabilities to test", func() {
		It("should not add any statuses to the tracker", func() {
			verifier.VerifyCapabilities(ctx, []string{})
			statuses := tracker.GetAllStatuses()
			Expect(statuses).To(BeEmpty())
		})
	})
})
