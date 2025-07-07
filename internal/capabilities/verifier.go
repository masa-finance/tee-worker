package capabilities

import (
	"context"
	"fmt"
	"time"

	"github.com/masa-finance/tee-worker/internal/capabilities/health"
	"github.com/sirupsen/logrus"
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

// VerifyCapabilities runs all the registered capability checks with panic recovery and timeout protection.
func (v *CapabilityVerifier) VerifyCapabilities(ctx context.Context, capabilities []string) {
	for _, cap := range capabilities {
		verifier, supported := v.verifiers[cap]
		if !supported {
			// If a capability is detected but not verifiable, we should mark it as unhealthy
			// This prevents advertising capabilities that we can't actually verify
			logrus.WithField("capability", cap).Warn("Capability detected but no verifier available - marking as unhealthy")
			v.tracker.UpdateStatus(cap, false, fmt.Errorf("no verifier available for capability: %s", cap))
			continue
		}

		// Verify capability with panic recovery and timeout protection
		v.verifyCapabilityWithProtection(ctx, cap, verifier)
	}
}

// verifyCapabilityWithProtection runs a single capability verification with panic recovery and timeout.
func (v *CapabilityVerifier) verifyCapabilityWithProtection(ctx context.Context, capability string, verifier Verifier) {
	// Create a timeout context for this verification (30 seconds)
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Channel to receive the result
	resultChan := make(chan struct {
		isHealthy bool
		err       error
	}, 1)

	// Run verification in a goroutine with panic recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.WithField("capability", capability).
					WithField("panic", r).
					Error("Capability verifier panicked")
				resultChan <- struct {
					isHealthy bool
					err       error
				}{false, fmt.Errorf("verifier panicked: %v", r)}
			}
		}()

		isHealthy, err := verifier.Verify(timeoutCtx)
		resultChan <- struct {
			isHealthy bool
			err       error
		}{isHealthy, err}
	}()

	// Wait for result or timeout
	select {
	case result := <-resultChan:
		v.tracker.UpdateStatus(capability, result.isHealthy, result.err)
	case <-timeoutCtx.Done():
		logrus.WithField("capability", capability).
			Warn("Capability verification timed out")
		v.tracker.UpdateStatus(capability, false, fmt.Errorf("verification timed out after 30 seconds"))
	}
}
