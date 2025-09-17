package llmapify_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWebApifyClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WebApify Client Suite")
}
