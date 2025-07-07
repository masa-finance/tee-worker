package jobserver_test

import (
	"context"
	_ "os"
	_ "time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/jobserver"
)

var _ = Describe("Jobserver", func() {
	It("runs jobs", func() {
		jobserver := NewJobServer(2, types.JobConfiguration{}, nil)

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
})
