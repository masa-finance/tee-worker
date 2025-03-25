package tee

import (
	"os"
	"testing"
)

func skipIfNotTEE(t *testing.T) {
	if os.Getenv("OE_SIMULATION") == "1" {
		t.Skip("Skipping TEE tests")
	}
}
