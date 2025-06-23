package jobs_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	teetypes "github.com/masa-finance/tee-types/types"
	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/internal/jobs"
	"github.com/masa-finance/tee-worker/internal/jobs/stats"
)

var _ = Describe("LinkedIn Scraper", func() {
	var linkedInScraper *LinkedInScraper
	var statsCollector *stats.StatsCollector

	BeforeEach(func() {
		logrus.SetLevel(logrus.DebugLevel)
		os.Setenv("LOG_LEVEL", "debug")

		// Check if LinkedIn credentials are set
		liAtCookie := os.Getenv("LINKEDIN_TEST_LI_AT_COOKIE")
		csrfToken := os.Getenv("LINKEDIN_TEST_CSRF_TOKEN")
		jsessionID := os.Getenv("LINKEDIN_TEST_JSESSIONID")

		if liAtCookie == "" || csrfToken == "" {
			Skip("LINKEDIN_TEST_LI_AT_COOKIE or LINKEDIN_TEST_CSRF_TOKEN is not set")
		}

		statsCollector = stats.StartCollector(128, types.JobConfiguration{})

		linkedInScraper = NewLinkedInScraper(types.JobConfiguration{
			"linkedin_li_at_cookie": liAtCookie,
			"linkedin_csrf_token":   csrfToken,
			"linkedin_jsessionid":   jsessionID,
		}, statsCollector)
	})

	Context("LinkedIn Profile Search", func() {
		It("should search profiles by query", func() {
			j := types.Job{
				Type: LinkedInScraperType,
				Arguments: map[string]interface{}{
					"type":        "searchbyquery",
					"query":       "software engineer",
					"max_results": 5,
				},
				WorkerID: "test-worker",
			}
			res, err := linkedInScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var results []*teetypes.LinkedInProfileResult
			err = res.Unmarshal(&results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).ToNot(BeEmpty())
			Expect(len(results)).To(BeNumerically("<=", 5))

			// Verify first profile has expected fields
			if len(results) > 0 {
				profile := results[0]
				Expect(profile.PublicIdentifier).ToNot(BeEmpty())
				Expect(profile.FullName).ToNot(BeEmpty())
				Expect(profile.Headline).ToNot(BeEmpty())
				Expect(profile.ProfileURL).To(ContainSubstring("linkedin.com/in/"))
			}

			// Check stats
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.LinkedInScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.LinkedInProfiles]).To(BeNumerically("==", uint(len(results))))
		})

		It("should search profiles with network filters", func() {
			j := types.Job{
				Type: LinkedInScraperType,
				Arguments: map[string]interface{}{
					"type":            "searchbyquery",
					"query":           "product manager",
					"network_filters": []string{"F", "S"}, // First and second degree connections
					"max_results":     3,
				},
				WorkerID: "test-worker",
			}
			res, err := linkedInScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var results []*teetypes.LinkedInProfileResult
			err = res.Unmarshal(&results)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).ToNot(BeEmpty())
			Expect(len(results)).To(BeNumerically("<=", 3))

			// Check stats
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.LinkedInScrapes]).To(BeNumerically("==", 1))
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.LinkedInProfiles]).To(BeNumerically("==", uint(len(results))))
		})

		It("should handle pagination with start parameter", func() {
			j := types.Job{
				Type: LinkedInScraperType,
				Arguments: map[string]interface{}{
					"type":        "searchbyquery",
					"query":       "data scientist",
					"max_results": 5,
					"start":       10, // Skip first 10 results
				},
				WorkerID: "test-worker",
			}
			res, err := linkedInScraper.ExecuteJob(j)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Error).To(BeEmpty())

			var results []*teetypes.LinkedInProfileResult
			err = res.Unmarshal(&results)
			Expect(err).NotTo(HaveOccurred())
			// Results might be empty if there aren't enough profiles
			Expect(err).NotTo(HaveOccurred())
		})

		It("should error on empty query", func() {
			j := types.Job{
				Type: LinkedInScraperType,
				Arguments: map[string]interface{}{
					"type":        "searchbyquery",
					"query":       "",
					"max_results": 10,
				},
				WorkerID: "test-worker",
			}
			res, err := linkedInScraper.ExecuteJob(j)
			Expect(err).To(HaveOccurred())
			Expect(res.Error).ToNot(BeEmpty())
			Expect(res.Error).To(ContainSubstring("query is required"))

			// Check error stats
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.LinkedInErrors]).To(BeNumerically("==", 1))
		})

		It("should error on unsupported query type", func() {
			j := types.Job{
				Type: LinkedInScraperType,
				Arguments: map[string]interface{}{
					"type":        "unsupported",
					"query":       "test",
					"max_results": 10,
				},
				WorkerID: "test-worker",
			}
			res, err := linkedInScraper.ExecuteJob(j)
			Expect(err).To(HaveOccurred())
			Expect(res.Error).ToNot(BeEmpty())
			Expect(res.Error).To(ContainSubstring("unsupported query type"))

			// Check error stats
			Expect(statsCollector.Stats.Stats[j.WorkerID][stats.LinkedInErrors]).To(BeNumerically("==", 1))
		})
	})

	Context("LinkedIn Authentication", func() {
		It("should error if no credentials are provided", func() {
			// Create scraper without credentials
			scraperNoAuth := NewLinkedInScraper(types.JobConfiguration{}, statsCollector)

			j := types.Job{
				Type: LinkedInScraperType,
				Arguments: map[string]interface{}{
					"type":        "searchbyquery",
					"query":       "test",
					"max_results": 10,
				},
				WorkerID: "test-worker",
			}
			res, err := scraperNoAuth.ExecuteJob(j)
			Expect(err).To(HaveOccurred())
			Expect(res.Error).ToNot(BeEmpty())
			Expect(res.Error).To(ContainSubstring("LinkedIn credentials not configured"))
		})
	})
})
