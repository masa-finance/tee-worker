package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/masa-finance/tee-worker/api/types"
	. "github.com/masa-finance/tee-worker/pkg/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	var (
		mockServer *httptest.Server
		client     *Client
	)

	BeforeEach(func() {
		mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/job/generate":
				if r.Method == http.MethodPost {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`mock-signature`))
				}
			case "/job/add":
				if r.Method == http.MethodPost {
					response := types.JobResponse{UID: "mock-job-id"}
					respJSON, _ := json.Marshal(response)
					w.WriteHeader(http.StatusOK)
					w.Write(respJSON)
				}
			case "/job/result":
				if r.Method == http.MethodPost {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`decrypted-result`))
				}
			case "/job/status/mock-job-id":
				if r.Method == http.MethodGet {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`encrypted-result`))
				}
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))

		var err error
		client, err = NewClient(mockServer.URL)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		mockServer.Close()
	})

	Describe("CreateJobSignature", func() {
		It("should create a job signature successfully", func() {
			job := types.Job{Type: "test-job"}
			signature, err := client.CreateJobSignature(job)
			Expect(err).NotTo(HaveOccurred())
			Expect(signature).To(Equal(JobSignature("mock-signature")))
		})
	})

	Describe("SubmitJob", func() {
		It("should submit a job successfully", func() {
			signature := JobSignature("mock-signature")
			jobResult, err := client.SubmitJob(signature)
			Expect(err).NotTo(HaveOccurred())
			Expect(jobResult.UUID).To(Equal("mock-job-id"))
		})
	})

	Describe("Decrypt", func() {
		It("should decrypt the encrypted result successfully", func() {
			signature := JobSignature("mock-signature")
			decryptedResult, err := client.Decrypt(signature, "mock-encrypted-result")
			Expect(err).NotTo(HaveOccurred())
			Expect(decryptedResult).To(Equal("decrypted-result"))
		})
	})

	Describe("GetResult", func() {
		It("should get the job result successfully", func() {
			result, found, err := client.GetResult("mock-job-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(result).To(Equal("encrypted-result"))
		})
	})
})
