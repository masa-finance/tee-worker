# tee-worker

Tee-worker is the Masa component to scrape data from a secure TEE enclave. It uses the [ego](https://github.com/edgelesssys/ego) framework to build, run and sign the binary.

Want to help in development? check the [DEVELOPMENT.md](DEVELOPMENT.md) file.

## Requirements

- Docker

## Run

To run the tee-worker, use docker with our images. Our images have signed binaries which are allowed to be part of the network:

```bash
docker run --device /dev/sgx_enclave --device /dev/sgx_provision --net host --rm -v $(PWD)/.masa:/home/masa -ti masaengineering/tee-worker:main
```

## Container images

All tagged images are available here: https://hub.docker.com/r/masaengineering/tee-worker/tags

- Images with `latest` tag are the latest releases
- Every branch has a corresponding image with the branch name (e.g. `main`)

### Docker compose

There is an example docker compose file to run the container with the appropriate environment variables.

```bash
docker-compose up
```

## API

The tee-worker exposes a simple http API to submit jobs, retrieve results, and decrypt the results.

```bash

### Submitting jobs
curl localhost:8080/job -H "Content-Type: application/json" -d '{ "type": "webscraper", "arguments": { "url": "google" } }'

### Jobs results
curl localhost:8080/job/b678ff77-118d-4a7a-a6ea-190eb850c28a

### Decrypt job results
curl localhost:8080/decrypt -H "Content-Type: application/json" -d '{ "encrypted_result": "'$result'" }'

```

### Golang client

It is available a simple golang client to interact with the API:

```golang
import(
    	. "github.com/masa-finance/tee-worker/pkg/client"
        "github.com/masa-finance/tee-worker/api/types"
)

func main() {
    clientInstance := NewClient(server.URL)

    // Step 1: Create the job request
    job := types.Job{
        Type: "web-scraper",
        Arguments: map[string]interface{}{
            "url": "google",
        },
    }

	// Step 2: Submit the job
	jobID, err := clientInstance.SubmitJob(job)

	// Step 3: Wait for the job result
	encryptedResult, err := clientInstance.WaitForResult(jobID, 10, time.Second)

	// Step 4: Decrypt the result
	decryptedResult, err := clientInstance.DecryptResult(encryptedResult)
}
```
