# Development

## Requirements

- Linux box
- OpenSSL
- Docker
- make


## Build

To build it is reccomended to use Docker. 

### Building the container image

To build the container image, use the appropriate Make targets:

```bash
make docker-build
```

This will generate a container image with a signed binary, and if there isn't a key available in `tee/private.pem` it will generate a new one.

## Running tests

A test suite is available, running inside container images with the appropriate environment and dependencies available (ego).

```bash
make test
```

## Running the container for development

To run the container, use the appropriate Make target(below), or you can use the container images published in [dockerhub](https://hub.docker.com/r/masaengineering/tee-worker/tags):

```bash
## Run without an Intel SGX hardware
docker run --net host -e STANDALONE=true -e OE_SIMULATION=1 --rm -v $PWD/.masa:/home/masa -ti masaengineering/tee-worker:main
```

### If you have an Intel-SGX capable machine

```bash
make run-sgx
```

### If you don't have an Intel-SGX capable machine

```bash
make run-simulate
```
