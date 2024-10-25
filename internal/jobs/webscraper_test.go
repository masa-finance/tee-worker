package jobs_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/jobs"
)

var _ = Describe("Webscraper", func() {
	It("should fake scraping for now", func() {
		webScraper := NewWebScraper()

		res, err := webScraper.ExecuteJob(types.Job{
			Type: "web-scraper",
			Arguments: map[string]interface{}{
				"url": "google",
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())
		Expect(res.Data.(string)).To(Equal("google"))
	})
})
