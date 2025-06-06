VERSION?=$(shell git describe --tags --abbrev=0)
PWD:=$(shell pwd)
IMAGE?=masa-tee-worker:latest
TEST_COOKIE_DIR?=$(PWD)/.testdir
export DISTRIBUTOR_PUBKEY?=$(shell cat tee/keybroker.pub | base64 -w0)

print-version:
	@echo "Version: ${VERSION}"

clean:
	@rm -rf bin

docker-compose-up:
	@docker compose up --build

build:
	@ego-go build -v -gcflags=all="-N -l" -ldflags '-linkmode=external -extldflags=-static' -ldflags "-X github.com/masa-finance/tee-worker/internal/versioning.ApplicationVersion=${VERSION} -X github.com/masa-finance/tee-worker/pkg/tee.KeyDistributorPubKey=${DISTRIBUTOR_PUBKEY}" -o ./bin/masa-tee-worker ./cmd/tee-worker

sign: tee/private.pem
	@ego sign ./tee/masa-tee-worker.json

ci-sign:
	@ego sign ./tee/masa-tee-worker.json

bundle:
	@ego bundle ./bin/masa-tee-worker

run-simulate: docker-build
	touch .masa/.env
	echo "STANDALONE=true" > .masa/.env
	@docker run --net host -e STANDALONE=true -e OE_SIMULATION=1 --rm -v $(PWD)/.masa:/home/masa -ti $(IMAGE)

run-sgx: docker-build
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
	docker build --build-arg DISTRIBUTOR_PUBKEY="$(DISTRIBUTOR_PUBKEY)" --secret id=private_key,src=./tee/private.pem  -t $(IMAGE) -f Dockerfile .

$(TEST_COOKIE_DIR):
	@mkdir -p $(TEST_COOKIE_DIR)

test: tee/private.pem $(TEST_COOKIE_DIR)
	@docker build --target=dependencies --build-arg baseimage=builder --secret id=private_key,src=./tee/private.pem -t $(IMAGE) -f Dockerfile .
	@docker run --user root -e TWITTER_TEST_ACCOUNT -e LOG_LEVEL=debug -e TEST_COOKIE_DIR=/cookies -v $(TEST_COOKIE_DIR):/cookies -v $(PWD)/coverage:/app/coverage --rm --workdir /app $(IMAGE) go test -coverprofile=coverage/coverage.txt -covermode=atomic -v ./...
