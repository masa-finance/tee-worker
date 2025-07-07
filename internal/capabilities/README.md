# Capabilities Package

The capabilities package provides a robust system for detecting, verifying, and monitoring the health of worker capabilities in the TEE worker system.

## Overview

This package solves the critical problem where workers advertise capabilities they don't actually possess, leading to job failures and network instability. It implements a comprehensive verification system that:

- **Detects** available capabilities based on configuration
- **Verifies** each capability at startup with real functionality tests
- **Monitors** capability health during runtime based on job execution results
- **Reconciles** unhealthy capabilities automatically in the background
- **Reports** real-time capability health via telemetry

## Architecture

### Core Components

#### 1. Detection (`detector.go`)
- **Purpose**: Discovers potential capabilities from job configuration
- **Input**: Job configuration with credentials and settings
- **Output**: List of detected capabilities + health tracker
- **Key Function**: `DetectCapabilities(ctx, jobConfig, jobServer)`

#### 2. Verification (`verifier.go`)
- **Purpose**: Tests each capability with real functionality checks
- **Features**: 
  - Panic recovery (prevents verifier crashes)
  - 30-second timeout protection
  - Parallel-safe verification
- **Key Function**: `VerifyCapabilities(ctx, capabilities)`

#### 3. Health Tracking (`health/`)
- **Purpose**: Maintains real-time health status of all capabilities
- **Features**:
  - Thread-safe status updates
  - Background reconciliation loop
  - Exponential backoff for failed capabilities
- **Key Interface**: `CapabilityHealthTracker`

#### 4. Verifiers (`verifiers/`)
- **Purpose**: Implement capability-specific verification logic
- **Available Verifiers**:
  - `WebScraperVerifier`: HTTP connectivity test
  - `TikTokVerifier`: API transcription test
  - `TwitterVerifier`: Search functionality test (credentials + API keys)
  - `LinkedInVerifier`: Profile fetch test

## Usage

### Basic Usage

```go
// Detect and verify capabilities at startup
healthyCapabilities, healthTracker := capabilities.DetectCapabilities(ctx, jobConfig, nil)

// Start background reconciliation
go healthTracker.StartReconciliationLoop(ctx)

// Update capability health based on job results
healthTracker.UpdateStatus("twitter-search", false, authError)
```

### Adding New Capabilities

1. **Create Verifier** (`verifiers/new_capability.go`):
```go
type NewCapabilityVerifier struct {
    // Configuration fields
}

func (v *NewCapabilityVerifier) Verify(ctx context.Context) (bool, error) {
    // Implement minimal functionality test
    return true, nil
}
```

2. **Register in Detector** (`detector.go`):
```go
// In DetectCapabilities function
verifiersMap["new-capability"] = verifiers.NewNewCapabilityVerifier(config)
```

3. **Add to Detection Logic** (`detectCapabilitiesFromConfig`):
```go
// Add capability to detected list when appropriate config is present
if hasNewCapabilityConfig {
    detected = append(detected, "new-capability")
}
```

## Configuration

### Supported Capabilities

| Capability | Required Config | Verifier Test |
|------------|----------------|---------------|
| `web-scraper` | None (always available) | HEAD request to example.com |
| `telemetry` | None (always available) | No verifier (marked unhealthy) |
| `tiktok-transcription` | None (always available) | Transcribe known video |
| `searchbyquery` | Twitter credentials/API keys | Search "BTC" query |
| `getbyid` | Twitter credentials/API keys | Same as searchbyquery |
| `getprofilebyid` | Twitter credentials/API keys | Same as searchbyquery |
| `getprofile` | LinkedIn credentials | Fetch Bill Gates profile |

### Configuration Examples

```go
// Twitter with credentials
jobConfig := types.JobConfiguration{
    "twitter_accounts": []string{"username:password"},
    "data_dir": "/tmp/twitter",
}

// Twitter with API keys
jobConfig := types.JobConfiguration{
    "twitter_api_keys": []string{"api_key_1", "api_key_2"},
}

// LinkedIn with credentials
jobConfig := types.JobConfiguration{
    "linkedin_credentials": []interface{}{
        map[string]interface{}{
            "li_at_cookie": "cookie_value",
            "csrf_token": "token_value", 
            "jsessionid": "session_value",
        },
    },
}
```

## System Behavior

### Startup Flow
1. **Detection**: Scan job configuration for potential capabilities
2. **Verification**: Test each capability with timeout and panic protection
3. **Filtering**: Only healthy capabilities are advertised to the network
4. **Reconciliation**: Start background loop to re-test unhealthy capabilities

### Runtime Flow
1. **Job Execution**: Jobs execute using healthy capabilities
2. **Health Updates**: Job failures update capability health status
3. **Dynamic Telemetry**: `/telemetry` endpoint reflects real-time health
4. **Auto-Recovery**: Background reconciliation heals capabilities when possible

### Error Handling
- **Verifier Panics**: Caught and logged, capability marked unhealthy
- **Timeouts**: 30-second limit prevents blocking, capability marked unhealthy
- **Network Issues**: Temporary failures trigger reconciliation attempts
- **Invalid Credentials**: Permanent failures until configuration changes

## Integration Points

### JobServer Integration
```go
// Pass health tracker to JobServer
jobServer := jobserver.NewJobServer(workers, config, healthTracker)
```

### Telemetry Integration
```go
// StatsCollector uses health tracker for dynamic capability reporting
statsCollector := stats.StartCollector(bufSize, config, healthTracker)
```

### Job Execution Integration
```go
// Jobs report outcomes to health tracker
if authError {
    healthTracker.UpdateStatus("twitter-search", false, authError)
}
```

## Testing

### Unit Tests
- `detector_test.go`: Capability detection logic
- `verifier_test.go`: Verification system with mocks
- `health/tracker_test.go`: Health tracking functionality

### Integration Tests
- End-to-end capability verification flow
- Network failure simulation
- Credential validation scenarios

## Known Issues & TODOs

### Current Issues
1. **Missing Verifiers**: `telemetry` capability has no verifier (marked unhealthy)
2. **Test Updates**: Tests need updating for new verification behavior
3. **LinkedIn Parsing**: Credential parsing could be more robust
4. **API Key Testing**: Twitter API key verification is placeholder

### Future Improvements
1. **Concurrent Verification**: Run verifications in parallel for faster startup
2. **Configurable Timeouts**: Allow per-capability timeout configuration
3. **Health Metrics**: Add Prometheus metrics for capability health
4. **Graceful Degradation**: Partial capability functionality when some features fail

## Security Considerations

- **Credential Exposure**: Verifiers use real credentials - ensure secure logging
- **Network Requests**: Verifiers make external API calls - consider rate limiting
- **Error Messages**: Avoid exposing sensitive information in error messages
- **Timeout Values**: Balance between thorough testing and startup speed

## Performance Impact

- **Startup Time**: Verification adds ~30 seconds maximum per capability
- **Memory Usage**: Health tracker maintains status for all capabilities
- **Network Usage**: Verifiers make minimal test requests
- **CPU Usage**: Background reconciliation runs with exponential backoff

## Troubleshooting

### Common Issues

**Capability Not Advertised**
- Check if verifier is registered in `detector.go`
- Verify configuration format matches expected structure
- Check logs for verification failures

**Startup Hanging**
- Verifier timeout protection should prevent this
- Check for network connectivity issues
- Review verifier implementation for blocking calls

**False Negatives**
- Verifier may be too strict for test environment
- Consider environment-specific test endpoints
- Check for temporary API issues

### Debug Commands

```bash
# Check capability health
curl http://localhost:8080/telemetry | jq '.reported_capabilities'

# Monitor verification logs
tail -f worker.log | grep -i "verif\|capabil"

# Test specific capability
go test ./internal/capabilities -v -run TestSpecificCapability
``` 