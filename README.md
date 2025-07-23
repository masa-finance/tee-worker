# tee-worker

Tee-worker is the Masa component to scrape data from a secure TEE enclave. It uses the [ego](https://github.com/edgelesssys/ego) Golang SDK to build, run and sign the binary for usage with Intel SGX.

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

The tee-worker requires various environment variables for operation. These should be set in `.masa/.env` (for Docker) or exported in your shell (for local runs). You can use `.env.example` as a reference.

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
- `JOB_TIMEOUT_SECONDS`: Maximum duration of a job when multiple calls are needed to get the number of results requested (default: `300`).

### Capabilities

**Capability Detection and Reporting:**

The worker automatically detects available capabilities based on:
- Twitter credentials (username:password pairs) - enables credential-based features
- Twitter API keys - enables API-based features  
- Available services (web scraper, TikTok transcription, telemetry)

The telemetry report includes all auto-detected capabilities, providing complete visibility of the worker's actual capabilities and ensuring transparency in resource allocation and worker evaluation within the MASA ecosystem.

**Job Types and Capabilities Structure:**

The worker uses a structured capability system where each **Job Type** has associated **sub-capabilities**. This is defined in `api/types/capabilities.go` and detected in `internal/capabilities/detector.go`.

**Main Job Types:**

Each job type represents a distinct service with its own set of capabilities:

1. **`web`** - Web scraping services
   - **Sub-capabilities**: `["web-scraper"]`
   - **Requirements**: None (always available)

2. **`telemetry`** - Worker monitoring and stats
   - **Sub-capabilities**: `["telemetry"]` 
   - **Requirements**: None (always available)

3. **`tiktok`** - TikTok video processing
   - **Sub-capabilities**: `["tiktok-transcription"]`
   - **Requirements**: None (always available)

4. **`twitter-credential`** - Twitter scraping with credentials
   - **Sub-capabilities**: `["searchbyquery", "searchbyfullarchive", "searchbyprofile", "getbyid", "getreplies", "getretweeters", "gettweets", "getmedia", "gethometweets", "getforyoutweets", "getprofilebyid", "gettrends", "getfollowing", "getfollowers", "getspace"]`
   - **Requirements**: `TWITTER_ACCOUNTS` environment variable

5. **`twitter-api`** - Twitter scraping with API keys
   - **Sub-capabilities**: `["searchbyquery", "getbyid", "getprofilebyid"]` (basic), plus `["searchbyfullarchive"]` for elevated API keys
   - **Requirements**: `TWITTER_API_KEYS` environment variable

6. **`twitter`** - General Twitter scraping (uses best available auth)
   - **Sub-capabilities**: Dynamic based on available authentication (same as credential or API depending on what's configured)
   - **Requirements**: Either `TWITTER_ACCOUNTS` or `TWITTER_API_KEYS`

**Twitter Sub-Capability Status:**

✅ **Working Sub-Capabilities (13):**
- `searchbyquery`, `searchbyfullarchive`, `searchbyprofile`
- `getbyid`, `getreplies`, `getretweeters` 
- `gettweets`, `getmedia`, `gethometweets`, `getforyoutweets`
- `getprofilebyid`, `gettrends`, `getfollowing`, `getfollowers`, `getspace`

**Capability Detection Logic:**

The system auto-detects capabilities based on environment configuration:
- If `TWITTER_ACCOUNTS` is set → enables `twitter-credential` and `twitter` job types
- If `TWITTER_API_KEYS` is set → enables `twitter-api` and `twitter` job types  
- If both are set → enables all three Twitter job types
- Core services (`web`, `telemetry`, `tiktok`) are always available

**API Job Types vs Capability Job Types:**

Note the distinction between:
- **API Job Types** (used in API calls): `twitter-scraper`, `twitter-credential-scraper`, `twitter-api-scraper`
- **Capability Job Types** (used in telemetry): `twitter`, `twitter-credential`, `twitter-api`

The API job types determine authentication behavior, while capability job types are used for capability reporting and detection.

**Sub-Capability Examples:**

Below are example job calls for each supported sub-capability:

**Web Scraping:**
```json
{
  "type": "web-scraper",
  "arguments": {
    "url": "https://www.google.com",
    "depth": 1
  }
}
```

**TikTok Transcription:**
```json
{
  "type": "tiktok-transcription", 
  "arguments": {
    "video_url": "https://www.tiktok.com/@coachty23/video/7502100651397172526",
    "language": "eng-US"
  }
}
```

**Twitter Sub-Capabilities:**

**Tweet Search Operations:**
```json
{
  "type": "twitter-scraper",
  "arguments": {
    "type": "searchbyquery",
    "query": "AI",
    "max_results": 2
  }
}

{
  "type": "twitter-api-scraper",
  "arguments": {
    "type": "searchbyfullarchive", 
    "query": "climate change",
    "max_results": 100
  }
}
```

**Single Tweet Operations:**
```json
{
  "type": "twitter-scraper",
  "arguments": {
    "type": "getbyid",
    "query": "1881258110712492142"
  }
}

{
  "type": "twitter-scraper", 
  "arguments": {
    "type": "getreplies",
    "query": "1234567890"
  }
}
```

**User Timeline Operations:**
```json
{
  "type": "twitter-scraper",
  "arguments": {
    "type": "gettweets",
    "query": "NASA", 
    "max_results": 5
  }
}

{
  "type": "twitter-scraper",
  "arguments": {
    "type": "getmedia",
    "query": "NASA",
    "max_results": 5
  }
}

{
  "type": "twitter-credential-scraper",
  "arguments": {
    "type": "gethometweets",
    "max_results": 5
  }
}

{
  "type": "twitter-credential-scraper", 
  "arguments": {
    "type": "getforyoutweets",
    "max_results": 5
  }
}
```

**Profile Operations:**
```json
{
  "type": "twitter-scraper",
  "arguments": {
    "type": "searchbyprofile",
    "query": "NASA_Marshall"
  }
}

{
  "type": "twitter-scraper",
  "arguments": {
    "type": "getprofilebyid", 
    "query": "44196397"
  }
}

{
  "type": "twitter-scraper",
  "arguments": {
    "type": "getfollowers",
    "query": "NASA"
  }
}

{
  "type": "twitter-scraper",
  "arguments": {
    "type": "getfollowing",
    "query": "NASA",
    "max_results": 5
  }
}

{
  "type": "twitter-scraper",
  "arguments": {
    "type": "getretweeters",
    "query": "1234567890",
    "max_results": 5
  }
}
```

**Other Operations:**
```json
{
  "type": "twitter-scraper",
  "arguments": {
    "type": "gettrends"
  }
}
```

**Telemetry:**
```json
{
  "type": "telemetry",
  "arguments": {}
}
```

See `.env.example` for more details.

## Container images

All tagged images are available here: https://hub.docker.com/r/masaengineering/tee-worker/tags

- Images with `latest` tag are the latest releases
- Every branch has a corresponding image with the branch name (e.g. `main`)

### Docker compose

There are two example docker compose file to run the container with the appropriate environment variables. They are similar but `docker-compose.yml` is meant as an example for using in production, while `docker-compose.dev.yml` is meant for testing.

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

### Health Check Endpoints

The service provides health check endpoints:

#### GET /healthz (Liveness Probe)
Returns HTTP 200 OK if the service is alive and running.

```bash
curl localhost:8080/healthz
```

Response:
```json
{
  "status": "ok",
  "service": "tee-worker"
}
```

#### GET /readyz (Readiness Probe)
Returns HTTP 200 OK if the service is ready to accept traffic. Returns HTTP 503 Service Unavailable if:
- The job server is not initialized
- The error rate exceeds 95% in the last 10 minutes

```bash
curl localhost:8080/readyz
```

Response when healthy:
```json
{
  "service": "tee-worker",
  "ready": true,
  "checks": {
    "job_server": "ok",
    "error_rate": "healthy",
    "stats": {
      "error_count": 5,
      "success_count": 95,
      "total_count": 100,
      "error_rate": 0.05,
      "window_start": "2024-01-15T10:00:00Z",
      "window_duration": "10m0s"
    }
  }
}
```

Response when unhealthy:
```json
{
  "service": "tee-worker",
  "ready": false,
  "checks": {
    "error_rate": "unhealthy",
    "stats": {
      "error_count": 96,
      "success_count": 4,
      "total_count": 100,
      "error_rate": 0.96,
      "window_start": "2024-01-15T10:00:00Z",
      "window_duration": "10m0s"
    }
  }
}
```

Note: Health check endpoints do not require API key authentication.

### Available Job Types
- `web-scraper`: Scrapes content from web pages
- `twitter-scraper`: General Twitter content scraping (uses best available auth method)
- `twitter-credential-scraper`: Forces Twitter credential-based scraping (requires `TWITTER_ACCOUNTS`)
- `twitter-api-scraper`: Forces Twitter API-based scraping (requires `TWITTER_API_KEYS`) 
- `tiktok-transcription`: Transcribes TikTok videos to text
- `telemetry`: Returns worker statistics and capabilities

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

#### Available Twitter scraping types
- `twitter-scraper`: General Twitter scraping (uses best available auth method)
- `twitter-credential-scraper`: Forces credential-based scraping (requires Twitter accounts)
- `twitter-api-scraper`: Forces API-based scraping (requires Twitter API keys)

Note: The worker will validate that the required authentication method is available for the chosen job type.

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

The tee-worker currently supports 6 job types:

**TODO:** Add descriptions of the return values.

#### `web-scraper`

Scrapes a URL down to some depth.

**Arguments**

* `url` (string): The URL to scrape.
* `depth` (int): How deep to go (if unset or less than 0, will be set to 1).

#### `twitter-scraper`, `twitter-credential-scraper`, `twitter-api-scraper`

Performs different types of Twitter searches using various authentication methods.

**Common Arguments**

* `type` (string): Type of query/operation (see capability examples below).
* `query` (string): The query to execute. Its meaning depends on the type of operation.
* `max_results` (int): How many results to return (optional, defaults vary by operation).
* `next_cursor` (string): Cursor for pagination (optional, supported by some operations).

**Supported Twitter Capabilities with Examples:**

**Tweet Search Operations:**

1. **`searchbyquery`** - Search tweets using Twitter API query syntax
   ```json
   {
     "type": "twitter-scraper",
     "arguments": {
       "type": "searchbyquery",
       "query": "climate change",
       "max_results": 10
     }
   }
   ```
   Returns: Array of `TweetResult` objects

2. **`searchbyfullarchive`** - Search full tweet archive (requires elevated API key for API-based scraping)
   ```json
   {
     "type": "twitter-api-scraper", 
     "arguments": {
       "type": "searchbyfullarchive",
       "query": "NASA",
       "max_results": 100
     }
   }
   ```
   Returns: Array of `TweetResult` objects

**Single Tweet Operations:**

3. **`getbyid`** - Get specific tweet by ID
   ```json
   {
     "type": "twitter-scraper",
     "arguments": {
       "type": "getbyid",
       "query": "1881258110712492142"
     }
   }
   ```
   Returns: Single `TweetResult` object

4. **`getreplies`** - Get replies to a specific tweet
   ```json
   {
     "type": "twitter-scraper",
     "arguments": {
       "type": "getreplies",
       "query": "1234567890",
       "max_results": 20
     }
   }
   ```
   Returns: Array of `TweetResult` objects

**User Timeline Operations:**

5. **`gettweets`** - Get tweets from a user's timeline
   ```json
   {
     "type": "twitter-scraper",
     "arguments": {
       "type": "gettweets",
       "query": "NASA",
       "max_results": 50
     }
   }
   ```
   Returns: Array of `TweetResult` objects

6. **`getmedia`** - Get media (photos/videos) from a user
   ```json
   {
     "type": "twitter-scraper",
     "arguments": {
       "type": "getmedia", 
       "query": "NASA",
       "max_results": 20
     }
   }
   ```
   Returns: Array of `TweetResult` objects with media

7. **`gethometweets`** - Get authenticated user's home timeline (credential-based only)
   ```json
   {
     "type": "twitter-credential-scraper",
     "arguments": {
       "type": "gethometweets",
       "max_results": 30
     }
   }
   ```
   Returns: Array of `TweetResult` objects

8. **`getforyoutweets`** - Get "For You" timeline (credential-based only)
   ```json
   {
     "type": "twitter-credential-scraper",
     "arguments": {
       "type": "getforyoutweets", 
       "max_results": 25
     }
   }
   ```
   Returns: Array of `TweetResult` objects

**Profile Operations:**

9. **`searchbyprofile`** - Get user profile information
   ```json
   {
     "type": "twitter-scraper",
     "arguments": {
       "type": "searchbyprofile",
       "query": "NASA_Marshall"
     }
   }
   ```
   Returns: `Profile` object

10. **`getprofilebyid`** - Get user profile by user ID
    ```json
    {
      "type": "twitter-scraper",
      "arguments": {
        "type": "getprofilebyid",
        "query": "44196397"
      }
    }
    ```
    Returns: `Profile` object

11. **`getfollowers`** - Get followers of a profile  
    ```json
    {
      "type": "twitter-scraper",
      "arguments": {
        "type": "getfollowers",
        "query": "NASA",
        "max_results": 100
      }
    }
    ```
    Returns: Array of `Profile` objects

12. **`getfollowing`** - Get users that a profile is following
    ```json
    {
      "type": "twitter-scraper", 
      "arguments": {
        "type": "getfollowing",
        "query": "NASA",
        "max_results": 100
      }
    }
    ```
    Returns: Array of `Profile` objects

13. **`getretweeters`** - Get users who retweeted a specific tweet
    ```json
    {
      "type": "twitter-scraper",
      "arguments": {
        "type": "getretweeters",
        "query": "1234567890",
        "max_results": 50
      }
    }
    ```
    Returns: Array of `Profile` objects

**Other Operations:**

14. **`gettrends`** - Get trending topics
    ```json
    {
      "type": "twitter-scraper",
      "arguments": {
        "type": "gettrends"
      }
    }
    ```
    Returns: Array of trending topic strings

**Note on Previously Unsupported Operations:**

The following Twitter operations have been removed from the worker as they were broken or unsupported:
- `searchfollowers` (use `getfollowers` instead)
- `getbookmarks` (was returning empty results)  
- `getspaces` (not implemented)

**Pagination Support:**

Some operations support cursor-based pagination using the `next_cursor` parameter:
- `gettweets`, `getmedia`, `gethometweets`, `getforyoutweets`, `getfollowers`
- Include `next_cursor` from previous response to get next page of results

**Complete Environment Configuration Example:**

```env
# Web scraping 
WEBSCRAPER_BLACKLIST="google.com,google.be"

# Twitter authentication (use one or both)
TWITTER_ACCOUNTS="user1:pass1,user2:pass2"  
TWITTER_API_KEYS="bearer_token1,bearer_token2"
TWITTER_SKIP_LOGIN_VERIFICATION="true"

# TikTok transcription
TIKTOK_DEFAULT_LANGUAGE="eng-US"

# Server configuration
LISTEN_ADDRESS=":8080"
API_KEY="your-secret-api-key"

# Caching and performance
RESULT_CACHE_MAX_SIZE=1000
RESULT_CACHE_MAX_AGE_SECONDS=600
JOB_TIMEOUT_SECONDS=300
```

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

## Development notes

If you add an environment variable, make sure that you also add it to `./tee/masa-tee-worker.json`. There is a CI test to ensure that all environment variables used are included in that file.

## Testing

You can run the unit tests using `make test`. If you need to do manual testing you can run `docker compose -f docker-compose.dev.yml up --build`. Once it's running you can use `curl` from another terminal window to send requests and check the responses (see the scraping examples above). To shut down use `docker compose -f docker-compose.dev.yml down`, or simply Ctrl+C.

If the tee-worker keeps crashing because your host does not support SGX emulation, i.e. some later Intel processors or Mac M-series, you can do one of the following.

### Testing remotely

If you have SSH access to a host that can support SGX emulation, you can instruct Docker to use a remote Docker daemon. For this, set the `DOCKER_HOST` environment variable to `ssh://<remote_host>`. You need to have SSH access via a private key (no password required). If you're using a shared host, you should copy `docker-compose.dev.yml` to a file that is not committed, rename the `masa-tee-worker` container to something else (e.g. appending your handle) and changing the `ports` specification to use a unique port (e.g. `8080:8081`) so you don't have conflicts with other users.

Since Docker does not support remote port forwarding, you will also have to run a separate SSH command to forward the listen port (set to 8080 in `docker-compose.dev.yml`, or changed above). If it's set to e.g. 8081, You can use `ssh -NT -L 8080:localhost:8081 <remote_host> &`. This will start an SSH command in the background that will forward port 8080 to the remote host.

To verify that everything is set up correctly, run `curl localhost:8080/readyz`. You should get a JSON reply with the tee-worker readiness status.

Once you're done with your testing remember to run `fg` and then Ctrl+C out of the SSH session.

### Using QEMU

You can also create a virtual machine using QEMU, and enable SGX emulation on it.

#### Some notes regarding M-series Macs

The TEE simulator does not work with Apple Virtualization. You will have to use QEMU (which will be very very slow, therefore it is preferred to use the option above). To use `docker compose` with the stack you will have to do the following:

* Do not use Docker Desktop. Install the `docker`, `docker-compose`, `lima` and `lima-additional-guestagents` Homebrew packages.
* Create the `colima` VM with the following command line:

``` bash
colima start --arch x86_64 --cpu-type max --cpu 2 --memory 4 --disk 60 --network-address --vm-type qemu

```

Or edit `$HOME/.colima/_templates/default.yaml`, modify the appropriate parameters and use `colima start`. The `--network-address` flag ensures that exposed ports are visible on the MacOS side, otherwise they will only be visible inside the Colima vm.

Once you have done this you can run `docker compose -f docker-compose.dev.yml up --build` without setting up `DOCKER_HOST`. Be aware that sometimes the Colima VM hangs, so you have to do `colima stop default` and `colima start default`. In extreme cases you might need to reboot your Mac.


 
