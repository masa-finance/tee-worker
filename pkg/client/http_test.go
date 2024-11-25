package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/masa-finance/tee-worker/pkg/client"
	. "github.com/masa-finance/tee-worker/pkg/client"

	"github.com/masa-finance/tee-worker/api/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	var (
		c          *Client
		server     *httptest.Server
		mockJobUID string
	)

	// Define a helper function to create a new test server
	setupServer := func(statusCode int, responseBody interface{}) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(statusCode)
			if responseBody != nil {
				json.NewEncoder(w).Encode(responseBody)
			}
		}))
	}

	BeforeEach(func() {
		mockJobUID = "mock-job-uid"
	})

	Context("SubmitJob", func() {
		var mockJob types.Job

		BeforeEach(func() {
			mockJob = types.Job{UUID: "job1"}
			server = setupServer(http.StatusOK, types.JobResponse{UID: mockJobUID})
			c = client.NewClient(server.URL)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should submit a job successfully", func() {
			jobResult, err := c.SubmitJob(mockJob)
			Expect(err).NotTo(HaveOccurred())
			Expect(jobResult.UUID).To(Equal(mockJobUID))
		})

		It("should return an error on HTTP failure", func() {
			server.Close() // close the server to simulate network error
			_, err := c.SubmitJob(mockJob)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("DecryptResult", func() {
		BeforeEach(func() {
			server = setupServer(http.StatusOK, "decrypted-result")
			c = client.NewClient(server.URL)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should decrypt result successfully", func() {
			result, err := c.Decrypt("encrypted-result")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal("\"decrypted-result\"\n"))
		})

		It("should return an error on decryption failure", func() {
			server.Close()
			server = setupServer(http.StatusInternalServerError, nil)
			c = client.NewClient(server.URL)
			_, err := c.Decrypt("encrypted-result")
			Expect(err).To(HaveOccurred())
		})
	})
})
