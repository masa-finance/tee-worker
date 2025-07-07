package capabilities

import (
	"context"

	"github.com/masa-finance/tee-worker/internal/capabilities/health"
)

// Verifier defines the interface for a capability verifier.
type Verifier interface {
	Verify(ctx context.Context) (bool, error)
}

// CapabilityVerifier is responsible for verifying the health of capabilities.
type CapabilityVerifier struct {
	tracker   health.CapabilityHealthTracker
	verifiers map[string]Verifier
}

// NewCapabilityVerifier creates a new instance of the CapabilityVerifier.
func NewCapabilityVerifier(tracker health.CapabilityHealthTracker) *CapabilityVerifier {
	return &CapabilityVerifier{
		tracker:   tracker,
		verifiers: make(map[string]Verifier),
	}
}

// RegisterVerifier adds a verifier for a specific capability.
func (v *CapabilityVerifier) RegisterVerifier(capability string, verifier Verifier) {
	v.verifiers[capability] = verifier
}

// VerifyCapabilities runs all the registered capability checks.
func (v *CapabilityVerifier) VerifyCapabilities(ctx context.Context, capabilities []string) {
	for _, cap := range capabilities {
		verifier, supported := v.verifiers[cap]
		if !supported {
			// If a capability is not explicitly verifiable, we can assume it's healthy
			// or decide on a default behavior. For now, we'll mark it as healthy.
			v.tracker.UpdateStatus(cap, true, nil)
			continue
		}

		isHealthy, err := verifier.Verify(ctx)
		v.tracker.UpdateStatus(cap, isHealthy, err)
	}
}
