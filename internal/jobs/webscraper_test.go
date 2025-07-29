package jobs_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
)

var statsCollector *stats.StatsCollector

var _ = Describe("Webscraper", func() {
	BeforeEach(func() {
		statsCollector = stats.StartCollector(128, types.JobConfiguration{})
	})

	It("should scrape now", func() {
		webScraper := NewWebScraper(types.JobConfiguration{}, statsCollector)

		j := types.Job{
			Type: teetypes.WebJob,
			Arguments: map[string]interface{}{
				"url": "https://www.google.com",
			},
			WorkerID: "test",
		}
		res, err := webScraper.ExecuteJob(j)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var scrapedData CollectedData
		res.Unmarshal(&scrapedData)
		Expect(err).NotTo(HaveOccurred())

		Expect(scrapedData.Pages).ToNot(BeEmpty())

		Eventually(func() uint {
			return statsCollector.Stats.Stats[j.WorkerID][stats.WebSuccess]
		}, 5*time.Second, 10*time.Millisecond).Should(BeNumerically("==", 1))
		Eventually(func() uint {
			return statsCollector.Stats.Stats[j.WorkerID][stats.WebErrors]
		}, 5*time.Second, 10*time.Millisecond).Should(BeNumerically("==", 0))
	})

	It("does not return data with invalid hosts", func() {
		webScraper := NewWebScraper(types.JobConfiguration{}, statsCollector)

		j := types.Job{
			Type: teetypes.WebJob,
			Arguments: map[string]interface{}{
				"url": "google",
			},
			WorkerID: "test",
		}
		res, err := webScraper.ExecuteJob(j)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Error).To(BeEmpty())

		var scrapedData CollectedData
		res.Unmarshal(&scrapedData)
		Expect(err).NotTo(HaveOccurred())

		Expect(scrapedData.Pages).To(BeEmpty())
		Eventually(func() uint {
			return statsCollector.Stats.Stats[j.WorkerID][stats.WebSuccess]
		}, 5*time.Second, 10*time.Millisecond).Should(BeNumerically("==", 1))
		Eventually(func() uint {
			return statsCollector.Stats.Stats[j.WorkerID][stats.WebErrors]
		}, 5*time.Second, 10*time.Millisecond).Should(BeNumerically("==", 0))
		Eventually(func() uint {
			return statsCollector.Stats.Stats[j.WorkerID][stats.WebInvalid]
		}, 5*time.Second, 10*time.Millisecond).Should(BeNumerically("==", 0))
	})

	It("should allow to blacklist urls", func() {
		webScraper := NewWebScraper(types.JobConfiguration{
			"webscraper_blacklist": []string{"google"},
		}, statsCollector)

		j := types.Job{
			Type: teetypes.WebJob,
			Arguments: map[string]interface{}{
				"url": "google",
			},
			WorkerID: "test",
		}
		res, err := webScraper.ExecuteJob(j)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Error).To(Equal("URL blacklisted: google"))
		Eventually(func() uint {
			return statsCollector.Stats.Stats[j.WorkerID][stats.WebSuccess]
		}, 5*time.Second, 10*time.Millisecond).Should(BeNumerically("==", 0))
		Eventually(func() uint {
			return statsCollector.Stats.Stats[j.WorkerID][stats.WebErrors]
		}, 5*time.Second, 10*time.Millisecond).Should(BeNumerically("==", 0))
		Eventually(func() uint {
			return statsCollector.Stats.Stats[j.WorkerID][stats.WebInvalid]
		}, 5*time.Second, 10*time.Millisecond).Should(BeNumerically("==", 1))
	})
})
