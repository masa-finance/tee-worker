package reddit_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestReddit(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reddit Suite")
}