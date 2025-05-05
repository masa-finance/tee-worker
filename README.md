
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

### New Login Skipping Feature

When you want to skip Twitter login verification (for development or specific deployment scenarios), you can set the `SKIP_LOGIN_VERIFICATION` environment variable:

```sh
export SKIP_LOGIN_VERIFICATION=true
make run
```

This will bypass the Twitter login verification process while still maintaining other security controls.

You can also specify specific Twitter credentials to skip verification using the `TWITTER_SKIP_VERIFICATION` environment variable:

```sh
export TWITTER_SKIP_VERIFICATION="foo:bar,baz:qux"
make run
```

This provides flexibility while maintaining security for different deployment scenarios.