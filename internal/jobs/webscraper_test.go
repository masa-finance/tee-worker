package jobs_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/jobs"
)

var _ = Describe("Webscraper", func() {
	It("should scrape now", func() {
		webScraper := NewWebScraper(types.JobConfiguration{})

		res, err := webScraper.ExecuteJob(types.Job{
			Type: WebScraperType,
			Arguments: map[string]interface{}{
				"url": "https://www.google.com",
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var scrapedData CollectedData
		res.Unmarshal(&scrapedData)
		Expect(err).NotTo(HaveOccurred())

		Expect(len(scrapedData.Pages)).ToNot(BeZero())
	})

	It("does not return data with invalid hosts", func() {
		webScraper := NewWebScraper(types.JobConfiguration{})

		res, err := webScraper.ExecuteJob(types.Job{
			Type: WebScraperType,
			Arguments: map[string]interface{}{
				"url": "google",
			},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var scrapedData CollectedData
		res.Unmarshal(&scrapedData)
		Expect(err).NotTo(HaveOccurred())

		Expect(len(scrapedData.Pages)).To(BeZero())
	})

	It("should allow to blacklist urls", func() {
		webScraper := NewWebScraper(types.JobConfiguration{
			"webscraper_blacklist": []string{"google"},
		})

		res, err := webScraper.ExecuteJob(types.Job{
			Type: WebScraperType,
			Arguments: map[string]interface{}{
				"url": "google",
			},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Error).To(Equal("URL blacklisted: google"))
	})
})
