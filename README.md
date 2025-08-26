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
- `APIFY_API_KEY`: API key for Apify Twitter scraping services. Required for `twitter-apify` job type and enables enhanced follower/following data collection.
- `LISTEN_ADDRESS`: The address the service listens on (default: `:8080`).
- `RESULT_CACHE_MAX_SIZE`: Maximum number of job results to keep in the result cache (default: `1000`).
- `RESULT_CACHE_MAX_AGE_SECONDS`: Maximum age (in seconds) to keep a result in the cache (default: `600`).
- `JOB_TIMEOUT_SECONDS`: Maximum duration of a job when multiple calls are needed to get the number of results requested (default: `300`).

## Capabilities

The worker automatically detects and exposes capabilities based on available configuration. Each capability is organized under a **Job Type** with specific **sub-capabilities**.

### Available Job Types and Capabilities

**Core Services (Always Available):**

1. **`web`** - Web scraping services
   - **Sub-capabilities**: `["scraper"]`
   - **Requirements**: None (always available)

2. **`tiktok`** - TikTok video processing
   - **Sub-capabilities**: `["transcription"]`
   - **Requirements**: None (always available)

3. **`reddit`** - Reddit scraping services
   - **Sub-capabilities**: `["scrapeurls","searchposts","searchusers","searchcommunities"]`
   - **Requirements**: `APIFY_API_KEY` environment variable

**Twitter Services (Configuration-Dependent):**

4. **`twitter-credential`** - Twitter scraping with credentials
   - **Sub-capabilities**: `["searchbyquery", "searchbyfullarchive", "searchbyprofile", "getbyid", "getreplies", "getretweeters", "gettweets", "getmedia", "gethometweets", "getforyoutweets", "getprofilebyid", "gettrends", "getfollowing", "getfollowers", "getspace"]`
   - **Requirements**: `TWITTER_ACCOUNTS` environment variable

5. **`twitter-api`** - Twitter scraping with API keys
   - **Sub-capabilities**: `["searchbyquery", "getbyid", "getprofilebyid"]` (basic), plus `["searchbyfullarchive"]` for elevated API keys
   - **Requirements**: `TWITTER_API_KEYS` environment variable

6. **`twitter`** - General Twitter scraping (uses best available auth)
   - **Sub-capabilities**: Dynamic based on available authentication (combines capabilities from credential, API, and Apify depending on what's configured)
   - **Requirements**: Either `TWITTER_ACCOUNTS`, `TWITTER_API_KEYS`, or `APIFY_API_KEY`
   - **Priority**: For follower/following operations: Apify > Credentials. For search operations: Credentials > API.

7. **`twitter-apify`** - Twitter scraping using Apify's API (requires `APIFY_API_KEY`)
   - **Sub-capabilities**: `["getfollowers", "getfollowing"]`
   - **Requirements**: `APIFY_API_KEY` environment variable

**Stats Service (Always Available):**

8. **`telemetry`** - Worker monitoring and stats
   - **Sub-capabilities**: `["telemetry"]`
   - **Requirements**: None (always available)

## API

The tee-worker exposes a simple HTTP API to submit jobs, retrieve results, and decrypt the results.

### Complete Request Flow

Here's the complete 4-step process for any job type:

```bash
# 1. Generate job signature
SIG=$(curl -s localhost:8080/job/generate \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${API_KEY}" \
  -d '{
    "type": "web",
    "arguments": {
      "url": "https://example.com",
      "depth": 1
    }
  }')

# 2. Submit the job
uuid=$(curl -s localhost:8080/job/add \
  -H "Content-Type: application/json" \
  -d '{ "encrypted_job": "'$SIG'" }' \
  | jq -r .uid)

# 3. Check job status (poll until complete)
result=$(curl -s localhost:8080/job/status/$uuid)

# 4. Decrypt job results
curl -s localhost:8080/job/result \
  -H "Content-Type: application/json" \
  -d '{
    "encrypted_result": "'$result'",
    "encrypted_request": "'$SIG'"
  }'
```

### Job Types and Parameters

All job types follow the same API flow above. Here are the available job types and their specific parameters:

#### `web`
Scrapes content from web pages.

**Parameters:**
- `url` (string, required): The URL to scrape
- `depth` (int, optional): How deep to go (defaults to 1 if unset or < 0)

```json
{
  "type": "web",
  "arguments": {
    "type": "scraper",
    "url": "https://www.google.com",
    "depth": 1
  }
}
```

#### `telemetry`
Returns worker statistics and capabilities. No parameters required.

```json
{
  "type": "telemetry",
  "arguments": {}
}
```

#### `tiktok-transcription`
Transcribes TikTok videos to text.

**Parameters:**
- `video_url` (string, required): The TikTok video URL to transcribe
- `language` (string, optional): Language for transcription (e.g., "eng-US"). Auto-detects if not specified.

**Returns:**
- `transcription_text`: The extracted text from the video
- `detected_language`: The language detected/used for transcription
- `video_title`: The title of the TikTok video
- `original_url`: The original video URL
- `thumbnail_url`: URL to the video thumbnail (if available)

```json
{
  "type": "tiktok",
  "arguments": {
    "type": "transcription",
    "video_url": "https://www.tiktok.com/@coachty23/video/7502100651397172526",
    "language": "eng-US"
  }
}
```

#### Reddit Job Types

There are four different types of Reddit searches:

- `scrapeurls`: Gets the content of one or more Reddit URLs
- `searchposts`: Searches posts and comments
- `searchusers`: Searches user profiles
- `searchcommunities`: Searches communities

**Parameters** (all are optional except where noted)

- `urls` (array of object with `url` and `query` keys, required for `scrapeurls`): Each element contains a Reddit URL to scrape together with the method (which by default will be `"GET"`).
- `queries` (array of string, required for all job types except `scrapeurls`): Each element is a string to search for. 
- `sort` (string) What to order by. Possible values are `"relevance"`, `"hot"`, `"top"`, `"new"`, `"rising"` and `"comments"`.
- `include_nsfw` (boolean): Whether to include content tagged NSFW. Default is `false`.
- `skip_posts`: (boolean): If `true`, `searchusers` will not return user posts. Default is `false`.
- `after`: (string, ISO8601 timestamp): Only return entries created after this date/time.
- `max_items` (nonnegative integer): How many items to load in the server cache (page through them using the cursor). Default is 10.
- `max_results` (nonnegative integer): How many results to return per page. Default is 10.
- `max_posts` (nonnegative integer): How many results to return per page. Default is 10.
- `max_comments` (nonnegative integer): How many results to return per page maximum. Default is 10.
- `max_communities` (nonnegative integer): How many results to return per page maximum. Default is 2.
- `max_users` (nonnegative integer): How many users to return per page maximum. Default is 2.
- `next_cursor` (string, optional): Pagination cursor.

##### Reddit Search Operations

**`scrapeurls`** - Scrape Reddit URLs

``` json
{
  "type": "reddit",
  "arguments": {
    "type": "scrapeurls",
    "urls": [
      {
        "url": "https://reddit.com/r/ArtificialIntelligence",
        "method": "GET"
      },
      {
        "url": "https://reddit.com/u/TheTelegraph"
      }
    ],
    "sort": "new",
    "include_nsfw": true,
    "max_items": 100
  }
}
```

**`searchusers`** - Search Reddit users

``` json
{
  "type": "reddit",
  "arguments": {
    "type": "searchusers",
    "queries": [
      "NASA",
      "European Space Agency"
    ],
    "sort": "relevance",
    "skip_posts": true,
  }
}
```

**`searchposts`** - Search Reddit posts

``` json
{
  "type": "reddit",
  "arguments": {
    "type": "searchposts",
    "queries": [
      "NASA",
      "European Space Agency"
    ],
    "max_items": 100,
    "max_results": 10,
    "max_posts": 5
  }
}
```

**`searchcommunities`** - Search Reddit posts

``` json
{
  "type": "reddit",
  "arguments": {
    "type": "searchcommunities",
    "queries": [
      "Artificial Intelligence"
    ],
    "max_items": 100,
    "max_results": 10,
  }
}
```

#### Twitter Job Types

Twitter scraping is available through four job types:
- `twitter`: Uses best available auth method (credential, API, or Apify)
- `twitter-credential`: Forces credential-based scraping (requires `TWITTER_ACCOUNTS`)
- `twitter-api`: Forces API-based scraping (requires `TWITTER_API_KEYS`)
- `twitter-apify`: Forces Apify-based scraping (requires `APIFY_API_KEY`)

**Common Parameters:**
- `type` (string, required): The operation type (see sub-capabilities below)
- `query` (string): The query to execute (meaning depends on operation type)
- `max_results` (int, optional): Number of results to return
- `next_cursor` (string, optional): Pagination cursor (supported by some operations)

##### Tweet Search Operations

**`searchbyquery`** - Search tweets using Twitter query syntax
```json
{
  "type": "twitter",
  "arguments": {
    "type": "searchbyquery",
    "query": "climate change",
    "max_results": 10
  }
}
```

**`searchbyfullarchive`** - Search full tweet archive (requires elevated API key for API-based scraping)
```json
{
  "type": "twitter-api",
  "arguments": {
    "type": "searchbyfullarchive",
    "query": "NASA",
    "max_results": 100
  }
}
```

**`getbyid`** - Get specific tweet by ID
```json
{
  "type": "twitter",
  "arguments": {
    "type": "getbyid",
    "query": "1881258110712492142"
  }
}
```

**`getreplies`** - Get replies to a specific tweet
```json
{
  "type": "twitter-credential",
  "arguments": {
    "type": "getreplies",
    "query": "1234567890",
    "max_results": 20
  }
}
```

**`getretweeters`** - Get users who retweeted a specific tweet
```json
{
  "type": "twitter-credential",
  "arguments": {
    "type": "getretweeters",
    "query": "1234567890",
    "max_results": 50
  }
}
```

##### User Timeline Operations

**`gettweets`** - Get tweets from a user's timeline
```json
{
  "type": "twitter-credential",
  "arguments": {
    "type": "gettweets",
    "query": "NASA",
    "max_results": 50
  }
}
```

**`getmedia`** - Get media (photos/videos) from a user
```json
{
  "type": "twitter-credential",
  "arguments": {
    "type": "getmedia",
    "query": "NASA",
    "max_results": 20
  }
}
```

**`gethometweets`** - Get authenticated user's home timeline (credential-based only)
```json
{
  "type": "twitter-credential",
  "arguments": {
    "type": "gethometweets",
    "max_results": 30
  }
}
```

**`getforyoutweets`** - Get "For You" timeline (credential-based only)
```json
{
  "type": "twitter-credential",
  "arguments": {
    "type": "getforyoutweets",
    "max_results": 25
  }
}
```

##### Profile Operations

**`searchbyprofile`** - Get user profile information
```json
{
  "type": "twitter-credential",
  "arguments": {
    "type": "searchbyprofile",
    "query": "NASA_Marshall"
  }
}
```

**`getprofilebyid`** - Get user profile by user ID
```json
{
  "type": "twitter",
  "arguments": {
    "type": "getprofilebyid",
    "query": "44196397"
  }
}
```

**`getfollowers`** - Get followers of a profile
```json
{
  "type": "twitter-credential",
  "arguments": {
    "type": "getfollowers",
    "query": "NASA",
    "max_results": 100
  }
}
```

**`getfollowers`** (using Apify for enhanced data) - Get followers with detailed profile information
```json
{
  "type": "twitter-apify",
  "arguments": {
    "type": "getfollowers",
    "query": "NASA",
    "max_results": 100,
    "next_cursor": "optional_pagination_cursor"
  }
}
```

**`getfollowing`** - Get users that a profile is following
```json
{
  "type": "twitter-credential",
  "arguments": {
    "type": "getfollowing",
    "query": "NASA",
    "max_results": 100
  }
}
```

**`getfollowing`** (using Apify for enhanced data) - Get following with detailed profile information
```json
{
  "type": "twitter-apify",
  "arguments": {
    "type": "getfollowing",
    "query": "NASA",
    "max_results": 100,
    "next_cursor": "optional_pagination_cursor"
  }
}
```

##### Other Operations

**`gettrends`** - Get trending topics (no query required)
```json
{
  "type": "twitter-credential",
  "arguments": {
    "type": "gettrends"
  }
}
```

##### Return Types

**Enhanced Profile Data with Apify**: When using `twitter-apify` for `getfollowers` or `getfollowing` operations, the response returns `ProfileResultApify` objects which include comprehensive profile information such as:
- Basic profile data (ID, name, screen name, location, description)
- Detailed follower/following counts and engagement metrics
- Profile appearance settings and colors
- Account verification and security status
- Privacy and interaction settings
- Business account information when available

This enhanced data provides richer insights compared to standard credential or API-based profile results.

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
        Type: "web",
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


 
