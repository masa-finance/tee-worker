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

		// Start the server
		go Start(ctx, "127.0.0.1:40912", "", true, types.JobConfiguration{})

		// Wait for the server to start
		Eventually(func() error {
			c := client.NewClient("http://localhost:40912")
			// Create a job signature for an empty job. Eventually it should succeed
			_, err := c.CreateJobSignature(types.Job{
				Type:      jobs.WebScraperType,
				Arguments: map[string]interface{}{},
			})
			return err
		}, 10*time.Second).Should(Succeed())

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

		// Step 2: Get a Job signature
		jobSignature, err := clientInstance.CreateJobSignature(job)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobSignature).NotTo(BeEmpty())

		// Step 3: Submit the job
		jobResult, err := clientInstance.SubmitJob(jobSignature)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobResult.UUID).NotTo(BeEmpty())

		// Step 4: Wait for the job result
		encryptedResult, err := jobResult.Get()
		Expect(err).NotTo(HaveOccurred())
		Expect(encryptedResult).NotTo(BeEmpty())

		// Step 5: Decrypt the result
		decryptedResult, err := clientInstance.Decrypt(jobSignature, encryptedResult)
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
		// Step 2: Get a Job signature
		jobSignature, err := clientInstance.CreateJobSignature(job)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobSignature).NotTo(BeEmpty())

		// Step 3: Submit the job
		jobResult, err := clientInstance.SubmitJob(jobSignature)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobResult).NotTo(BeNil())

		// Step 4: Wait for the job result
		encryptedResult, err := jobResult.Get()
		Expect(err).NotTo(HaveOccurred())
		Expect(encryptedResult).NotTo(BeEmpty())

		// Step 5: Decrypt the result
		decryptedResult, err := clientInstance.Decrypt(jobSignature, encryptedResult)
		Expect(err).NotTo(HaveOccurred())

		Expect(decryptedResult).NotTo(BeEmpty())
		Expect(decryptedResult).To(ContainSubstring("google"))

		result, err := jobResult.GetDecrypted(jobSignature)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).NotTo(BeEmpty())
		Expect(result).To(ContainSubstring("google"))
	})

	It("bubble up errors", func() {
		// Step 1: Create the job request
		job := types.Job{
			Type: "not-existing scraper",
			Arguments: map[string]interface{}{
				"url": "google",
			},
		}

		// Step 2: Get a Job signature
		jobSignature, err := clientInstance.CreateJobSignature(job)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobSignature).NotTo(BeEmpty())

		// Step 3: Submit the job
		jobResult, err := clientInstance.SubmitJob(jobSignature)
		Expect(err).NotTo(HaveOccurred())
		Expect(jobResult).NotTo(BeNil())
		Expect(jobResult.UUID).NotTo(BeEmpty())

		jobResult.SetMaxRetries(10)

		// Step 4: Wait for the job result (should fail)
		encryptedResult, err := jobResult.Get()
		Expect(err).To(HaveOccurred())
		Expect(encryptedResult).To(BeEmpty())
	})
})
