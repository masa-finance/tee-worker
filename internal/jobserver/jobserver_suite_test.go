package jobserver_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestJobServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "JobServer test suite")
	RunSpecs(t, "ResultCache Suite")
}
