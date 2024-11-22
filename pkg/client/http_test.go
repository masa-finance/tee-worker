package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

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
			uid, err := c.SubmitJob(mockJob)
			Expect(err).NotTo(HaveOccurred())
			Expect(uid).To(Equal(mockJobUID))
		})

		It("should return an error on HTTP failure", func() {
			server.Close() // close the server to simulate network error
			_, err := c.SubmitJob(mockJob)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("GetJobResult", func() {
		BeforeEach(func() {
			server = setupServer(http.StatusOK, "encrypted-result")
			c = client.NewClient(server.URL)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should retrieve the job result successfully", func() {
			result, err := c.GetJobResult("job1")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal("\"encrypted-result\"\n"))
		})

		It("should return an error when job is not found", func() {
			server.Close()
			server = setupServer(http.StatusNotFound, types.JobError{Error: "job not found"})
			c = client.NewClient(server.URL)
			_, err := c.GetJobResult("invalid-job-id")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("job not found"))
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
			result, err := c.DecryptResult("encrypted-result")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal("\"decrypted-result\"\n"))
		})

		It("should return an error on decryption failure", func() {
			server.Close()
			server = setupServer(http.StatusInternalServerError, nil)
			c = client.NewClient(server.URL)
			_, err := c.DecryptResult("encrypted-result")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("WaitForResult", func() {
		BeforeEach(func() {
			server = setupServer(http.StatusOK, "encrypted-result")
			c = client.NewClient(server.URL)
		})

		AfterEach(func() {
			server.Close()
		})

		It("should wait and retrieve result successfully", func() {
			result, err := c.WaitForResult("job1", 3, time.Millisecond*10)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal("\"encrypted-result\"\n"))
		})

		It("should fail after max retries", func() {
			server.Close() // simulate unavailability
			result, err := c.WaitForResult("job1", 3, time.Millisecond*10)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("max retries reached"))
			Expect(result).To(BeEmpty())
		})
	})
})
