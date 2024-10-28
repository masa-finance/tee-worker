package main

import (
	"context"

	"github.com/masa-finance/tee-worker/internal/api"
)

func main() {
	api.Start(context.Background(), ":8080")
}
