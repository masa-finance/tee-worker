# tee-worker

Tee-worker is the Masa component to scrape data from a secure TEE enclave. It uses the [ego](https://github.com/edgelesssys/ego) framework to build, run and sign the binary.

Want to help in development? check the [DEVELOPMENT.md](DEVELOPMENT.md) file.

## Requirements

- Docker

## API Key Authentication

### Enabling API Key Protection

To require all API requests to supply an API key, set the `API_KEY` environment variable before starting the tee-worker:

```sh
export API_KEY=your-secret-key
make run
```

If `API_KEY` is not set, authentication is disabled and all requests are allowed (for development/local use).

### How it Works
- The server checks for the API key in the `Authorization: Bearer <API_KEY>` header (preferred) or the `X-API-Key` header.
- If the key is missing or incorrect, the server returns `401 Unauthorized`.

### Go Client Usage Example

```go
import "github.com/masa-finance/tee-worker/pkg/client"

cli := client.NewClient("http://localhost:8080", client.APIKey("your-secret-key"))
// All requests will now include the Authorization: Bearer header automatically.
```

## Run

To run the tee-worker, use docker with our images. Our images have signed binaries which are allowed to be part of the network:

```bash
mkdir .masa
wget https://raw.githubusercontent.com/masa-finance/tee-worker/refs/heads/main/.env.example -O .masa/.env
# Edit .masa/.env with your settings

# Run the worker
docker run --device /dev/sgx_enclave --device /dev/sgx_provision --net host --rm -v $(PWD)/.masa:/home/masa -ti masaengineering/tee-worker:main
```

## Credentials & Environment Variables

The tee-worker requires various environment variables for operation. These should be set in `.masa/.env` (for Docker) or exported in your shell (for local runs).

- `API_KEY`: (Optional) API key required for authenticating all HTTP requests to the tee-worker API. If set, all requests must include this key in the `Authorization: Bearer <API_KEY>` or `X-API-Key` header.
- `WEBSCRAPER_BLACKLIST`: Comma-separated list of domains to block for web scraping.
- `TWITTER_ACCOUNTS`: Comma-separated list of Twitter credentials in `username:password` format.
- `TWITTER_API_KEYS`: Comma-separated list of Twitter Bearer API tokens.
- `TWITTER_SKIP_LOGIN_VERIFICATION`: Set to `true` to skip Twitter's login verification step. This can help avoid rate limiting issues with Twitter's verify_credentials API endpoint when running multiple workers or processing large volumes of requests.
- `TIKTOK_DEFAULT_LANGUAGE`: Default language for TikTok transcriptions (default: `eng-US`).
- `TIKTOK_API_USER_AGENT`: User-Agent header for TikTok API requests (default: standard mobile browser user agent).
- `LISTEN_ADDRESS`: The address the service listens on (default: `:8080`).
- `RESULT_CACHE_MAX_SIZE`: Maximum number of job results to keep in the result cache (default: `1000`).
- `RESULT_CACHE_MAX_AGE_SECONDS`: Maximum age (in seconds) to keep a result in the cache (default: `600`).
- `CAPABILITIES`: Comma-separated list of capabilities to enable for the worker. This is a security feature to limit the actions the worker can perform. The default is `*` which allows all actions. If not set, the worker will automatically determine the capabilities (auto-detection) based on the provided Twitter credentials and API keys.  Note that this is an optional feature and it will override the capabilities that were set by the auto-detection.
- `JOB_TIMEOUT_SECONDS`: Maximum duration of a job when multiple calls are needed to get the number of results requested (default: `300`).

### Capabilities

The `CAPABILITIES` environment variable defines the actions the worker can perform. This is a security feature to limit the actions the worker can perform. The default is `*` which allows all actions.

Note that this is an optional feature. If not set, the worker will automatically determine the capabilities based on the provided Twitter credentials and API keys.

- `*`: All capabilities (default).
- `all`: All capabilities. Same as `*`.
- `searchbyquery`: Search by query. 
- `searchbyfullarchive`: Search by full archive. Only available for API keys with full archive access.
- `searchbyprofile`: Search by profile. 
- `searchfollowers`: Search followers.
- `getbyid`: Get by ID.
- `getreplies`: Get replies.
- `getretweeters`: Get retweeters.
- `gettweets`: Get tweets.
- `getmedia`: Get media.
- `gethometweets`: Get home tweets.
- `getforyoutweets`: Get "For You" tweets.
- `getbookmarks`: Get bookmarks.
- `getprofilebyid`: Get profile by ID.
- `gettrends`: Get trends.
- `getfollowing`: Get following.
- `getfollowers`: Get followers.
- `getspace`: Get space.
- `getspaces`: Get spaces.

See `.env.example` for more details.

**Example:**
```env
WEBSCRAPER_BLACKLIST="google.com,google.be"
TWITTER_ACCOUNTS="foo:bar,foo:baz"
TWITTER_API_KEYS="apikey1,apikey2"
TWITTER_SKIP_LOGIN_VERIFICATION="true"
LISTEN_ADDRESS=":8080"
RESULT_CACHE_MAX_SIZE=1000
RESULT_CACHE_MAX_AGE_SECONDS=600
CAPABILITIES="searchbyfullarchive,searchbyquery,searchbyprofile,searchfollowers,getbyid,getreplies,getretweeters,gettweets,getmedia,gethometweets,getforyoutweets,getbookmarks,getprofilebyid,gettrends,getfollowing,getfollowers,getspace,getspaces"
```

See `.env.example` for more details.

## Container images

All tagged images are available here: https://hub.docker.com/r/masaengineering/tee-worker/tags

- Images with `latest` tag are the latest releases
- Every branch has a corresponding image with the branch name (e.g. `main`)

### Docker compose

There is an example docker compose file to run the container with the appropriate environment variables.

```bash
docker-compose up
```

### Testing Mode

For testing outside a TEE environment:

```go
// Enable standalone mode
tee.SealStandaloneMode = true

// Create a new key ring and add a key for standalone mode (32 bytes for AES-256)
keyRing := tee.NewKeyRing()
keyRing.Add("0123456789abcdef0123456789abcdef")

// Set as the current key ring
tee.CurrentKeyRing = keyRing
```

### Important Notes

1. All encryption keys must be exactly 32 bytes long for AES-256 encryption
   - The system validates that keys are exactly 32 bytes (256 bits) when added through the `SetKey` function
   - An error will be returned if the key length is invalid
   - Example valid key: `"0123456789abcdef0123456789abcdef"` (32 bytes)
2. The sealing mechanism uses the TEE's product key in production mode
3. Key rings help manage multiple encryption keys and support key rotation
4. Salt-based key derivation adds an extra layer of security by deriving unique keys for different contexts
5. **Security Enhancement**: The keyring is now limited to a maximum of 2 keys per worker
   - This restriction prevents job recycling and potential replay attacks
   - Workers with more than 2 keys will be automatically pruned to the 2 most recent keys
   - The system enforces this limit when adding new keys and during startup validation

## API

The tee-worker exposes a simple HTTP API to submit jobs, retrieve results, and decrypt the results.

### Available Scraper Types
- `web-scraper`: Scrapes content from web pages
- `twitter-scraper`: General Twitter content scraping
- `twitter-credential-scraper`: Authenticated Twitter scraping
- `twitter-api-scraper`: Uses Twitter API for data collection

### Example 1: Web Scraper

```bash
# 1. Generate job signature for web scraping
SIG=$(curl localhost:8080/job/generate \
  -H "Content-Type: application/json" \
  -d '{
    "type": "web-scraper", 
    "arguments": { 
      "url": "https://example.com" 
    }
  }')

# 2. Submit the job
uuid=$(curl localhost:8080/job/add \
  -H "Content-Type: application/json" \
  -d '{ "encrypted_job": "'$SIG'" }' \
  | jq -r .uid)

# 3. Check job status
result=$(curl localhost:8080/job/status/$uuid)

# 4. Decrypt job results
curl localhost:8080/job/result \
  -H "Content-Type: application/json" \
  -d '{
    "encrypted_result": "'$result'", 
    "encrypted_request": "'$SIG'" 
  }'
```

### Example 2: Twitter API Scraping

#### Available twitter scraping types
- `twitter-scraper`: General Twitter scraping
- `twitter-credential-scraper`: Authenticated Twitter scraping
- `twitter-api-scraper`: Uses Twitter API for data collection

Note that the job argument types are the same as capabilities. The worker will check if the job type is allowed for the current worker.

```bash
# 1. Generate job signature for Twitter scraping
SIG=$(curl -s "localhost:8080/job/generate" \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "type": "twitter-api-scraper",
    "arguments": {
      "type": "searchbyfullarchive",
      "query": "climate change",
      "max_results": 100
    }
  }')

# 2. Submit the job
uuid=$(curl localhost:8080/job/add \
  -H "Content-Type: application/json" \
  -d '{ "encrypted_job": "'$SIG'" }' \
  | jq -r .uid)

# 3. Check job status
result=$(curl localhost:8080/job/status/$uuid)

# 4. Decrypt job results
curl localhost:8080/job/result \
  -H "Content-Type: application/json" \
  -d '{
    "encrypted_result": "'$result'", 
    "encrypted_request": "'$SIG'" 
  }'
```

### Example 3: Twitter Credential Scraping

```bash
# 1. Generate job signature for Twitter credential scraping
SIG=$(curl -s "localhost:8080/job/generate" \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "type": "twitter-credential-scraper",
    "arguments": {
      "type": "searchbyquery",
      "query": "climate change",
      "max_results": 10
    }
  }')

# 2. Submit the job
uuid=$(curl localhost:8080/job/add \
  -H "Content-Type: application/json" \
  -d '{ "encrypted_job": "'$SIG'" }' \
  | jq -r .uid)

# 3. Check job status
result=$(curl localhost:8080/job/status/$uuid)

# 4. Decrypt job results
curl localhost:8080/job/result \
  -H "Content-Type: application/json" \
  -d '{
    "encrypted_result": "'$result'", 
    "encrypted_request": "'$SIG'" 
  }'
```

### Example 4: TikTok Transcription

```bash
# 1. Generate job signature for TikTok transcription
SIG=$(curl -s "localhost:8080/job/generate" \
  -H "Authorization: Bearer ${AUTH_TOKEN}" \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "type": "tiktok-transcription",
    "arguments": {
      "video_url": "https://www.tiktok.com/@example/video/1234567890",
      "language": "eng-US"
    }
  }')

# 2. Submit the job
uuid=$(curl localhost:8080/job/add \
  -H "Content-Type: application/json" \
  -d '{ "encrypted_job": "'$SIG'" }' \
  | jq -r .uid)

# 3. Check job status
result=$(curl localhost:8080/job/status/$uuid)

# 4. Decrypt job results
curl localhost:8080/job/result \
  -H "Content-Type: application/json" \
  -d '{
    "encrypted_result": "'$result'", 
    "encrypted_request": "'$SIG'" 
  }'
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

The tee-worker currently supports 4 job types:

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
* `max_results` (int): How many results to return.
* `next_cursor` (int): Cursor returned from the previous query, for pagination (for those job types that support it).

**Job types**

Some job types now support cursor-based pagination. For these jobs:

- The get variants ignore the next_cursor parameter and retrieve the first `max_results` records quickly
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

#### `twitter-credential-scraper`
- **Description:**
  - Like `twitter-scraper`, but **forces the use of Twitter credentials** (username/password) for scraping. Twitter API keys will not be used for these jobs.
- **Arguments:**
  - Same as `twitter-scraper`.
- **Returns:**
  - Same as `twitter-scraper`.

#### `twitter-api-scraper`
- **Description:**
  - Like `twitter-scraper`, but **forces the use of Twitter API keys** for scraping. Twitter credentials will not be used for these jobs.
- **Arguments:**
  - Same as `twitter-scraper`.
- **Returns:**
  - Same as `twitter-scraper`.

#### `tiktok-transcription`

Transcribes TikTok videos and extracts text from them.

**Arguments**

* `video_url` (string): The TikTok video URL to transcribe.
* `language` (string, optional): The desired language for transcription (e.g., "eng-US"). If not specified, uses the configured default or auto-detects.

**Returns**

* `transcription_text` (string): The extracted text from the video
* `detected_language` (string): The language detected/used for transcription
* `video_title` (string): The title of the TikTok video
* `original_url` (string): The original video URL
* `thumbnail_url` (string): URL to the video thumbnail (if available)

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

## Profiling

The tee-worker supports profiling via `pprof`. The TEE does not allow for profiling, so it can only be enabled when running in standalone mode.

There are two ways to enable profiling:

* Set `ENABLE_PPROF` to `true`.
* Send a POST request to `/debug/pprof/enable` (no body necessary)

There is currently no way to completely disable profiling short of restarting the tee-worker. However, you can send a POST request to `/debug/pprof/disable` which will disable the most resource-intensive probes (goroutine blocking, mutexes and CPU).

When profiling is enabled you will have access to the following endpoints, which you can use with the `go tool pprof` command:

`/debug/pprof` - Index page
`/debug/pprof/heap` - Heap profile
`/debug/pprof/goroutine` - Goroutine profile
`/debug/pprof/profile?seconds=XX` - CPU profile during XX seconds
`/debug/pprof/block` - Goroutine blocking
`/debug/pprof/mutex` - Holders of contended mutexes

There are others, see the `/debug/pprof` index page for a complete list.

The `/debug/pprof/trace?seconds=XX` will give you an XX-second execution trace, which you can use via the `go tool trace` command.

For more information, see [the official docs](https://pkg.go.dev/net/http/pprof). [This link](https://gist.github.com/andrewhodel/ed7625a14eb87404cafd37493849d1ba) also contains useful information.
