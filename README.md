
- `SKIP_LOGIN_VERIFICATION`: (Optional) When set to `true`, skips Twitter login verification process. Useful for development or specific deployment scenarios where login verification is not required. Default is `false`.

### Updated Configuration Variables

The following new configuration variables have been added:

- `TWITTER_SKIP_VERIFICATION`: (Optional) Comma-separated list of Twitter credentials that should skip verification. Format: `username:password`. Default is empty.

These variables provide flexibility in authentication requirements while maintaining security controls.