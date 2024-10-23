VERSION?=$(shell git describe --tags --abbrev=0)
PWD:=$(shell pwd)
IMAGE?=masa-tee-worker:latest

print-version:
	@echo "Version: ${VERSION}"

clean:
	@rm -rf bin

docker-compose-up:
	@docker compose up --build

build:
	@ego-go build -v -gcflags=all="-N -l" -ldflags '-linkmode=external -extldflags=-static' -ldflags "-X github.com/masa-finance/tee-worker/internal/versioning.ApplicationVersion=${VERSION}" -o ./bin/masa-tee-worker ./cmd/tee-worker

sign: tee/private.pem
	@ego sign ./tee/masa-tee-worker.json

bundle:
	@ego bundle ./bin/masa-tee-worker

run-simulate: docker-build
	@docker run --net host -e OE_SIMULATION=1 --rm -v $(PWD)/.masa:/home/masa -ti $(IMAGE)

run-sgx: docker-build
	@docker run --device /dev/sgx_enclave --device /dev/sgx_provision --net host --rm -v $(PWD)/.masa:/home/masa -ti $(IMAGE)

## TEE bits
tee/private.pem:
	@openssl genrsa -out tee/private.pem -3 3072

docker-build: tee/private.pem
	@docker build --secret id=private_key,src=./tee/private.pem  -t $(IMAGE) -f Dockerfile .

test: tee/private.pem
	@docker build --build-arg baseimage=builder --secret id=private_key,src=./tee/private.pem -t $(IMAGE) -f Dockerfile .
	@docker run --user root -v $(PWD)/coverage:/app/coverage --rm --workdir /app -ti $(IMAGE) go test -coverprofile=coverage/coverage.txt -covermode=atomic -v ./...
