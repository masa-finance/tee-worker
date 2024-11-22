package api_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/api"
	"github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/pkg/client"
)

var _ = Describe("API", func() {

	var (
		clientInstance *client.Client
		ctx            context.Context
		cancel         context.CancelFunc
	)

	BeforeEach(func() {
		// Start the server
		ctx, cancel = context.WithCancel(context.Background())

		go Start(ctx, "127.0.0.1:40912", types.JobConfiguration{})

		// Wait for the server to start
		time.Sleep(2 * time.Second)

		// Initialize the client
		clientInstance = client.NewClient("http://localhost:40912")
	})

	AfterEach(func() {
		// Stop the server
		cancel()
	})

	It("should submit an invalid job, and fail because of the malformed URL. no results containing google", func() {
		// Step 1: Create the job request
		job := types.Job{
			Type: jobs.WebScraperType,
			Arguments: map[string]interface{}{
				"url": "google",
			},
		}

		// Step 2: Submit the job
		jobID, err := clientInstance.SubmitJob(job)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobID).NotTo(BeEmpty())

		// Step 3: Wait for the job result
		encryptedResult, err := clientInstance.WaitForResult(jobID, 10, time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(encryptedResult).NotTo(BeEmpty())

		// Step 4: Decrypt the result
		decryptedResult, err := clientInstance.DecryptResult(encryptedResult)
		Expect(err).NotTo(HaveOccurred())
		Expect(decryptedResult).NotTo(BeEmpty())
		Expect(decryptedResult).NotTo(ContainSubstring("google"))
		Expect(decryptedResult).To(ContainSubstring(`"pages":null`))
	})

	It("should submit a job and get the correct result", func() {
		// Step 1: Create the job request
		job := types.Job{
			Type: jobs.WebScraperType,
			Arguments: map[string]interface{}{
				"url":   "https://google.com",
				"depth": 1,
			},
		}

		// Step 2: Submit the job
		jobID, err := clientInstance.SubmitJob(job)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobID).NotTo(BeEmpty())

		// Step 3: Wait for the job result
		encryptedResult, err := clientInstance.WaitForResult(jobID, 10, time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(encryptedResult).NotTo(BeEmpty())

		// Step 4: Decrypt the result
		decryptedResult, err := clientInstance.DecryptResult(encryptedResult)
		Expect(err).NotTo(HaveOccurred())

		Expect(decryptedResult).NotTo(BeEmpty())
		Expect(decryptedResult).To(ContainSubstring("google"))
	})

	It("bubble up errors", func() {
		// Step 1: Create the job request
		job := types.Job{
			Type: "not-existing scraper",
			Arguments: map[string]interface{}{
				"url": "google",
			},
		}

		// Step 2: Submit the job
		jobID, err := clientInstance.SubmitJob(job)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobID).NotTo(BeEmpty())

		// Step 3: Wait for the job result (should fail)
		encryptedResult, err := clientInstance.WaitForResult(jobID, 10, time.Second)
		Expect(err).To(HaveOccurred())
		Expect(encryptedResult).To(BeEmpty())
	})
})
