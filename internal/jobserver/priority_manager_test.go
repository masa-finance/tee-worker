package jobserver_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/internal/jobserver"
)

var _ = Describe("PriorityManager", func() {
	var pm *jobserver.PriorityManager

	BeforeEach(func() {
		// Create priority manager without external endpoint (uses dummy data)
		pm = jobserver.NewPriorityManager("", 5*time.Minute)
	})

	AfterEach(func() {
		pm.Stop()
	})

	Describe("Worker Priority Check", func() {
		It("should identify priority workers from dummy list", func() {
			// These workers are in the dummy list
			Expect(pm.IsPriorityWorker("worker-001")).To(BeTrue())
			Expect(pm.IsPriorityWorker("worker-002")).To(BeTrue())
			Expect(pm.IsPriorityWorker("worker-005")).To(BeTrue())
			Expect(pm.IsPriorityWorker("worker-priority-1")).To(BeTrue())
			Expect(pm.IsPriorityWorker("worker-priority-2")).To(BeTrue())
			Expect(pm.IsPriorityWorker("worker-vip-1")).To(BeTrue())

			// These workers are not in the dummy list
			Expect(pm.IsPriorityWorker("worker-003")).To(BeFalse())
			Expect(pm.IsPriorityWorker("worker-004")).To(BeFalse())
			Expect(pm.IsPriorityWorker("random-worker")).To(BeFalse())
		})
	})

	Describe("Get Priority Workers", func() {
		It("should return the list of priority workers", func() {
			workers := pm.GetPriorityWorkers()
			Expect(workers).To(ContainElements(
				"worker-001",
				"worker-002",
				"worker-005",
				"worker-priority-1",
				"worker-priority-2",
				"worker-vip-1",
			))
			Expect(len(workers)).To(BeNumerically(">=", 6))
		})
	})

	Describe("Update Priority Workers", func() {
		It("should update the priority workers list", func() {
			newWorkers := []string{"new-worker-1", "new-worker-2", "new-worker-3"}
			pm.UpdatePriorityWorkers(newWorkers)

			// Old workers should no longer be priority
			Expect(pm.IsPriorityWorker("worker-001")).To(BeFalse())
			Expect(pm.IsPriorityWorker("worker-002")).To(BeFalse())

			// New workers should be priority
			Expect(pm.IsPriorityWorker("new-worker-1")).To(BeTrue())
			Expect(pm.IsPriorityWorker("new-worker-2")).To(BeTrue())
			Expect(pm.IsPriorityWorker("new-worker-3")).To(BeTrue())

			// Verify the list
			workers := pm.GetPriorityWorkers()
			Expect(workers).To(ConsistOf("new-worker-1", "new-worker-2", "new-worker-3"))
		})
	})

	Describe("With External Endpoint", func() {
		It("should fall back to dummy data when endpoint fails", func() {
			// Create manager with non-existent external endpoint
			// This will fail to fetch and fall back to dummy data
			pmWithEndpoint := jobserver.NewPriorityManager("https://api.example.com/priority-workers", 5*time.Minute)
			defer pmWithEndpoint.Stop()

			// Should have dummy workers after failed fetch
			workers := pmWithEndpoint.GetPriorityWorkers()
			Expect(len(workers)).To(BeNumerically(">", 0))

			// Check for expected dummy workers
			Expect(pmWithEndpoint.IsPriorityWorker("worker-001")).To(BeTrue())
			Expect(pmWithEndpoint.IsPriorityWorker("worker-priority-1")).To(BeTrue())
			Expect(pmWithEndpoint.IsPriorityWorker("worker-vip-1")).To(BeTrue())
		})
	})

	Describe("Refresh Interval", func() {
		It("should use default refresh interval when not specified", func() {
			pmDefault := jobserver.NewPriorityManager("", 0)
			defer pmDefault.Stop()

			// Should still have dummy workers
			Expect(pmDefault.IsPriorityWorker("worker-001")).To(BeTrue())
		})
	})
})