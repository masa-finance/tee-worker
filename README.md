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
SIG=$(curl localhost:8080/job/generate -H "Content-Type: application/json" -d '{ "type": "web-scraper", "arguments": { "url": "google" } }')

### Submitting jobs
uuid=$(curl localhost:8080/job/add -H "Content-Type: application/json" -d '{ "encrypted_job": "'$SIG'" }' | jq -r .uid)

### Jobs results
result=$(curl localhost:8080/job/status/$uuid)

### Decrypt job results
curl localhost:8080/job/result -H "Content-Type: application/json" -d '{ "encrypted_result": "'$result'", "encrypted_request": "'$SIG'" }'
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
            "url": "https://google.com",
            "depth": 1,
        },
    }

    // Step 2: Get a Job signature. Send the signature somewhere to be executed, or execute it locally (see below)
    jobSignature, err := clientInstance.CreateJobSignature(job) 

	// Step 3: Submit the job signature for execution ( can be done locally or remotely )
	jobResult, err := clientInstance.SubmitJob(jobSignature)

    // Step 4a: Get the job result (decrypted)
    result, err := jobResult.GetDecrypted(jobSignature)

    // Alternatively, you can get the encrypted result and decrypt it later
    // Note: this can be forwarded to another party to decrypt the result

	// Step 4b.1: Get the job result (encrypted)
	encryptedResult, err := jobResult.Get()

    // Step 4b.2: Decrypt the result
    decryptedResult, err := clientInstance.Decrypt(jobSignature, encryptedResult)
}
```

### Job types

The tee-worker currently supports 3 job types:

**TODO:** Add descriptions of the return values.

#### `web-scraper`

Scrapes a URL down to some depth.

**Arguments**

* `url` (string): The URL to scrape.
* `depth` (int): How deep to go (if unset or less than 0, will be set to 1).

#### `twitter-scraper`

Performs different types of Twitter searches.

**Arguments**

* `type` (string): Type of query (see below).
* `query` (string): The query to execute. Its meaning depends on the type of query (see below)
* `count` (int): How many results to return.
* `next_cursor` (int): Cursor returned from the previous query, for pagination (for those job types that support it).

**Job types**

Some job types now support cursor-based pagination. For these jobs:

- The get variants ignore the next_cursor parameter and retrieve the first count records quickly
- To paginate, first use an empty next_cursor to get initial results, then use the returned next_cursor in subsequent calls.

**Jobs that return tweets or lists of tweets**

* `searchbyquery` - Executes a query and returns the tweets that match. The `query` parameter is a query using the [Twitter API query syntax](https://developer.x.com/en/docs/x-api/v1/tweets/search/guides/standard-operators)
* `getbyid` - Returns a tweet given its ID. The `query` parameter is the tweet ID.
* `getreplies` - Returns a list of all the replies to a given tweet. The `query` parameter is the tweet ID.
* `gettweets`  - Returns all the tweets for a given profile. The `query` parameter is the profile to search.
* `gethometweets`  - Returns all the tweets from a profile's home timeline. The `query` parameter is the profile to search.
* `getforyoutweets` - Returns all the tweets from a profile's "For You" timeline. The `query` parameter is the profile to search.
* `getbookmarks`  - Returns all of a profile's bookmarked tweets. The `query` parameter is the profile to search.

**Jobs that return profiles or lists of profiles**

* `getprofilebyid` / `searchbyprofile` - Returns a given user profile. The `query` parameter is the profile to search for.
* `getfollowers` / `searchfollowers`  - Returns a list of profiles of the followers of a given profile. The `query` parameter is the profile to search.
* `getfollowing` - Returns all of the profiles a profile is following. The `query` parameter is the profile to search.
* `getretweeters` - Returns a list of profiles that have retweeted a given tweet. The `query` parameter is the tweet ID.

**Jobs that return other types of data**

* `getmedia` - Returns info about all the photos and videos for a given user. The `query` parameter is the profile to search.
* `gettrends`- Returns a list of all the trending topics. The `query` parameter is ignored.
* `getspace`- Returns info regarding a Twitter Space given its ID. The `query` parameter is the space ID.

#### `telemetry`

This job type has no parameters, and returns the current state of the worker. It returns an object with the following fields. All timestamps are given in local time, in seconds since the Unix epoch (1/1/1970 00:00:00 UTC). The counts represent the interval between the `boot_time` and the `current_time`. All the fields in the `stats` object are optional (if they are missing it means that its value is 0).

Note that the stats are reset whenever the node is rebooted (therefore we need the `boot_time` to properly account for the stats)

These are the fields in the response:

* `boot_time` - Timestamp when the process started up.
* `last_operation_time` - Timestamp when the last operation happened.
* `current_time` - Current timestamp of the host.
* `stats.twitter_scrapes` - Total number of Twitter scrapes.
* `stats.twitter_returned_tweets` - Number of tweets returned to clients (this does not consider other types of data such as profiles or trending topics).
* `stats.twitter_returned_profiles` - Number of profiles returned to clients.
* `stats.twitter_returned_other` - Number of other records returned to clients (e.g. media, spaces or trending topics).
* `stats.twitter_errors` - Number of errors while scraping tweets (excluding authentication and rate-limiting).
* `stats.twitter_ratelimit_errors` - Number of Twitter rate-limiting errors.
* `stats.twitter_auth_errors` - Number of Twitter authentication errors.
* `stats.web_success` - Number of successful web scrapes.
* `stats.web_errors` - Number of web scrapes that resulted in an error.
* `stats.web_invalid` - Number of invalid web scrape requests (at the moment, blacklisted domains).
