package tee

import (
	"os"
	"testing"
)

func skipIfNotTEE(t *testing.T) {
	if os.Getenv("TEE_ENABLED") != "true" {
		t.Skip("Skipping test in non-TEE environment")
	}
}
