package jobserver_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/jobserver"
)

var _ = Describe("Jobserver", func() {
	It("runs jobs", func() {
		jobserver := NewJobServer(2)

		uuid := jobserver.AddJob(types.Job{
			Type: "web-scraper",
			Arguments: map[string]interface{}{
				"url": "google",
			},
		})

		Expect(uuid).ToNot(BeEmpty())

		_, exists := jobserver.GetJobResult(uuid)
		Expect(exists).ToNot(BeTrue())

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go jobserver.Run(ctx)

		Eventually(func() bool {
			result, exists := jobserver.GetJobResult(uuid)
			return exists && result.Error == "" && result.Data.(string) == "google"
		}, "5s").Should(Not(BeNil()))
	})
})
