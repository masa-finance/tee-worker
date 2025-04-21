package jobserver

import (
	"testing"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestResultCache(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ResultCache Suite")
}
