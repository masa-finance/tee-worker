package health_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/masa-finance/tee-worker/internal/capabilities/health"
)

var _ = Describe("Tracker", func() {
	var tracker *Tracker

	BeforeEach(func() {
		tracker = NewTracker()
	})

	Describe("Creating a new tracker", func() {
		It("should not be nil and should have an initialized statuses map", func() {
			Expect(tracker).NotTo(BeNil())
			Expect(tracker.GetAllStatuses()).NotTo(BeNil())
		})
	})

	Describe("Updating capability status", func() {
		const capabilityName = "test-capability"
		var testError = fmt.Errorf("test error")

		Context("with an initial healthy status", func() {
			It("should correctly set the status to healthy", func() {
				tracker.UpdateStatus(capabilityName, true, nil)
				status, exists := tracker.GetStatus(capabilityName)
				Expect(exists).To(BeTrue())
				Expect(status.IsHealthy).To(BeTrue())
				Expect(status.LastError).To(BeEmpty())
				Expect(status.ErrorCount).To(BeZero())
				Expect(status.LastChecked).To(BeTemporally("~", time.Now(), time.Second))
			})
		})

		Context("when a capability becomes unhealthy", func() {
			It("should update the status, record the error, and increment the error count", func() {
				tracker.UpdateStatus(capabilityName, true, nil) // Start healthy
				tracker.UpdateStatus(capabilityName, false, testError)

				status, _ := tracker.GetStatus(capabilityName)
				Expect(status.IsHealthy).To(BeFalse())
				Expect(status.LastError).To(Equal(testError.Error()))
				Expect(status.ErrorCount).To(Equal(1))
			})
		})

		Context("when an unhealthy capability fails again", func() {
			It("should increment the error count", func() {
				tracker.UpdateStatus(capabilityName, false, testError)
				tracker.UpdateStatus(capabilityName, false, testError)

				status, _ := tracker.GetStatus(capabilityName)
				Expect(status.ErrorCount).To(Equal(2))
			})
		})

		Context("when a capability recovers to a healthy state", func() {
			It("should reset the error count and error message", func() {
				tracker.UpdateStatus(capabilityName, false, testError) // Start unhealthy
				tracker.UpdateStatus(capabilityName, true, nil)

				status, _ := tracker.GetStatus(capabilityName)
				Expect(status.IsHealthy).To(BeTrue())
				Expect(status.LastError).To(BeEmpty())
				Expect(status.ErrorCount).To(BeZero())
			})
		})
	})

	Describe("Getting capability status", func() {
		It("should return exists=false for a non-existent capability", func() {
			_, exists := tracker.GetStatus("non-existent")
			Expect(exists).To(BeFalse())
		})

		It("should return the correct status for an existing capability", func() {
			tracker.UpdateStatus("existing", true, nil)
			status, exists := tracker.GetStatus("existing")
			Expect(exists).To(BeTrue())
			Expect(status.Name).To(Equal("existing"))
		})
	})

	Describe("Getting all statuses", func() {
		It("should return all tracked statuses", func() {
			tracker.UpdateStatus("cap1", true, nil)
			tracker.UpdateStatus("cap2", false, fmt.Errorf("error"))
			statuses := tracker.GetAllStatuses()
			Expect(statuses).To(HaveLen(2))
			Expect(statuses).To(HaveKey("cap1"))
			Expect(statuses).To(HaveKey("cap2"))
		})

		It("should return a copy of the statuses map, not a reference", func() {
			tracker.UpdateStatus("cap1", true, nil)
			allStatuses := tracker.GetAllStatuses()

			// Modify the returned map
			allStatuses["cap1"] = CapabilityStatus{Name: "modified"}

			// Get the original status again and check it hasn't changed
			currentStatus, _ := tracker.GetStatus("cap1")
			Expect(currentStatus.Name).To(Equal("cap1"))
		})
	})
})
