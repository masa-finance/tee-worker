VERSION?=$(shell git describe --tags --abbrev=0)
PWD:=$(shell pwd)
IMAGE?=masa-tee-worker:latest
TEST_IMAGE?=$(IMAGE)
export DISTRIBUTOR_PUBKEY?=$(shell cat tee/keybroker.pub | base64 -w0)
export MINERS_WHITE_LIST?=
# Additional test arguments, e.g. TEST_ARGS="./internal/jobs" or TEST_ARGS="-v -run TestSpecific ./internal/capabilities"
export TEST_ARGS?=./...

# Helper to conditionally add --env-file if .env exists
ENV_FILE_ARG = $(shell [ -f .env ] && echo "--env-file $(PWD)/.env" || echo "")

print-version:
	@echo "Version: ${VERSION}"

clean:
	@rm -rf bin

docker-compose-up:
	@docker compose up --build

build:
	@ego-go build -v -gcflags=all="-N -l" -ldflags '-linkmode=external -extldflags=-static' -ldflags "-X github.com/masa-finance/tee-worker/internal/versioning.ApplicationVersion=${VERSION} -X github.com/masa-finance/tee-worker/pkg/tee.KeyDistributorPubKey=${DISTRIBUTOR_PUBKEY} -X github.com/masa-finance/tee-worker/internal/config.MinersWhiteList=${MINERS_WHITE_LIST}" -o ./bin/masa-tee-worker ./cmd/tee-worker

sign: tee/private.pem
	@ego sign ./tee/masa-tee-worker.json

ci-sign:
	@ego sign ./tee/masa-tee-worker.json

bundle:
	@ego bundle ./bin/masa-tee-worker

run-simulate: docker-build
	@mkdir -p .masa
	@[ ! -f .masa/.env ] && echo "STANDALONE=true" > .masa/.env || true
	@docker run --net host -e STANDALONE=true -e OE_SIMULATION=1 --rm -v $(PWD)/.masa:/home/masa -ti $(IMAGE)

run-sgx: docker-build
	@mkdir -p .masa
	@[ ! -f .masa/.env ] && echo "STANDALONE=true" > .masa/.env || true
	@docker run --device /dev/sgx_enclave --device /dev/sgx_provision --net host --rm -v $(PWD)/.masa:/home/masa -ti $(IMAGE)

## TEE bits
tee/private.pem:
	@openssl genrsa -out tee/private.pem -3 3072

tee/public.pub:
	@openssl rsa -in tee/private.pem -pubout -out tee/public.pem

tee/keybroker.pem:
	@openssl genrsa -out tee/keybroker.pem -3 4092

tee/keybroker.pub: tee/keybroker.pem
	@openssl rsa -in tee/keybroker.pem -outform PEM -pubout -out tee/keybroker.pub

docker-build: tee/private.pem
	docker build --build-arg DISTRIBUTOR_PUBKEY="$(DISTRIBUTOR_PUBKEY)" --build-arg MINERS_WHITE_LIST="$(MINERS_WHITE_LIST)" --secret id=private_key,src=./tee/private.pem  -t $(IMAGE) -f Dockerfile .

docker-build-test: tee/private.pem
	@docker build --target=dependencies --build-arg baseimage=builder --secret id=private_key,src=./tee/private.pem -t $(TEST_IMAGE) -f Dockerfile .

ci-test:
	go test -coverprofile=coverage/coverage.txt -covermode=atomic -v $(TEST_ARGS)

.PHONY: test
test: docker-build-test
	docker run --user root $(ENV_FILE_ARG) -e LOG_LEVEL=debug -v $(PWD)/coverage:/app/coverage --rm --workdir /app $(TEST_IMAGE) go test -coverprofile=coverage/coverage.txt -covermode=atomic -v $(TEST_ARGS)

test-capabilities: docker-build-test
	@docker run --user root $(ENV_FILE_ARG) -e LOG_LEVEL=debug -v $(PWD)/coverage:/app/coverage --rm --workdir /app $(TEST_IMAGE) go test -v ./internal/capabilities

test-api: docker-build-test
	@docker run --user root $(ENV_FILE_ARG) -v $(PWD)/.masa:/home/masa -v $(PWD)/coverage:/app/coverage --rm --workdir /app -e DATA_DIR=/home/masa $(TEST_IMAGE) go test -v ./internal/api

test-jobs: docker-build-test
	@docker run --user root $(ENV_FILE_ARG) -v $(PWD)/.masa:/home/masa -v $(PWD)/coverage:/app/coverage --rm --workdir /app -e DATA_DIR=/home/masa $(TEST_IMAGE) go test -v ./internal/jobs

test-twitter: docker-build-test
	@docker run --user root $(ENV_FILE_ARG) -v $(PWD)/.masa:/home/masa -v $(PWD)/coverage:/app/coverage --rm --workdir /app -e DATA_DIR=/home/masa $(TEST_IMAGE) go test -v ./internal/jobs/twitter_test.go ./internal/jobs/jobs_suite_test.go

test-tiktok: docker-build-test
	@docker run --user root $(ENV_FILE_ARG) -v $(PWD)/.masa:/home/masa -v $(PWD)/coverage:/app/coverage --rm --workdir /app -e DATA_DIR=/home/masa $(TEST_IMAGE) go test -v ./internal/jobs/tiktok_test.go ./internal/jobs/jobs_suite_test.go

test-reddit: docker-build-test
	@docker run --user root $(ENV_FILE_ARG) -v $(PWD)/.masa:/home/masa -v $(PWD)/coverage:/app/coverage --rm --workdir /app -e DATA_DIR=/home/masa $(TEST_IMAGE) go test -v ./internal/jobs/reddit_test.go ./internal/jobs/redditapify/client_test.go ./api/types/reddit/reddit_suite_test.go

test-web: docker-build-test
	@docker run --user root $(ENV_FILE_ARG) -v $(PWD)/.masa:/home/masa -v $(PWD)/coverage:/app/coverage --rm --workdir /app -e DATA_DIR=/home/masa $(TEST_IMAGE) go test -v ./internal/jobs/web_test.go ./internal/jobs/webapify/client_test.go ./internal/jobs/jobs_suite_test.go

test-telemetry: docker-build-test
	@docker run --user root $(ENV_FILE_ARG) -v $(PWD)/.masa:/home/masa -v $(PWD)/coverage:/app/coverage --rm --workdir /app -e DATA_DIR=/home/masa $(TEST_IMAGE) go test -v ./internal/jobs/telemetry_test.go