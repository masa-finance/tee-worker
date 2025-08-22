package jobserver_test

import (
	"context"
	_ "os"
	_ "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types"
	"github.com/masa-finance/tee-worker/internal/config"
	. "github.com/masa-finance/tee-worker/internal/jobserver"
)

var _ = Describe("Jobserver", func() {
	BeforeEach(func() {
		config.MinersWhiteList = ""
	})

	It("runs jobs", func() {
		jobserver := NewJobServer(2, types.JobConfiguration{})

		uuid, err := jobserver.AddJob(types.Job{
			Type: "web-scraper",
			Arguments: map[string]interface{}{
				"url": "google",
			},
		})

		Expect(uuid).ToNot(BeEmpty())
		Expect(err).ToNot(HaveOccurred())

		_, exists := jobserver.GetJobResult(uuid)
		Expect(exists).ToNot(BeTrue())

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go jobserver.Run(ctx)

		Eventually(func() bool {
			result, exists := jobserver.GetJobResult(uuid)
			return exists && result.Error == "" && string(result.Data) == "google"
		}, "5s").Should(Not(BeNil()))
	})
	It("whitelists miners", func() {
		config.MinersWhiteList = "miner1,miner2"
		jobserver := NewJobServer(2, types.JobConfiguration{})

		uuid, err := jobserver.AddJob(types.Job{
			Type: "web-scraper",
			Arguments: map[string]interface{}{
				"url": "google",
			},
			Nonce:    "1234567890",
			WorkerID: "miner3",
		})

		Expect(uuid).To(BeEmpty())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("this job is not from a whitelisted miner"))

		uuid, err = jobserver.AddJob(types.Job{
			Type:     "web-scraper",
			WorkerID: "miner1",
			Arguments: map[string]interface{}{
				"url": "google",
			},
			Nonce: "1234567891",
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(uuid).ToNot(BeEmpty())

		_, exists := jobserver.GetJobResult(uuid)
		Expect(exists).ToNot(BeTrue())
	})
	It("won't execute same jobs twice", func() {
		jobserver := NewJobServer(2, types.JobConfiguration{})

		uuid, err := jobserver.AddJob(types.Job{
			Type: "web-scraper",
			Arguments: map[string]interface{}{
				"url": "google",
			},
			Nonce:    "1234567890",
			WorkerID: "miner3",
		})

		Expect(uuid).ToNot(BeEmpty())
		Expect(err).ToNot(HaveOccurred())

		_, exists := jobserver.GetJobResult(uuid)
		Expect(exists).ToNot(BeTrue())

		uuid, err = jobserver.AddJob(types.Job{
			Type: "web-scraper",
			Arguments: map[string]interface{}{
				"url": "google",
			},
			Nonce: "1234567890",
		})
		Expect(uuid).To(BeEmpty())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("job already executed"))
	})
})
