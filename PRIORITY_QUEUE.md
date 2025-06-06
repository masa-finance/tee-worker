# Priority Queue System Documentation

## Overview

The tee-worker now implements a priority queue system that enables preferential processing of jobs from specific worker IDs. This system ensures that high-priority workers get their jobs processed faster while maintaining fair processing for all workers.

## Architecture

### Components

1. **Dual Queue System**
   - **Fast Queue**: For jobs from priority worker IDs
   - **Slow Queue**: For jobs from regular worker IDs

2. **Priority Manager**
   - Maintains a list of priority worker IDs
   - Fetches updates from an external endpoint
   - Refreshes the list periodically (default: 15 minutes)

3. **Job Router**
   - Routes incoming jobs to appropriate queues based on worker ID
   - Falls back to slow queue if fast queue is full

4. **Worker Processing**
   - Workers always check fast queue first
   - Only process slow queue jobs when fast queue is empty

## Configuration

Configure the priority queue system using these environment variables:

```bash
# Enable/disable priority queue system (default: true)
ENABLE_PRIORITY_QUEUE=true

# Queue sizes
FAST_QUEUE_SIZE=1000    # Max jobs in fast queue (default: 1000)
SLOW_QUEUE_SIZE=5000    # Max jobs in slow queue (default: 5000)

# External endpoint for priority worker list
EXTERNAL_WORKER_ID_PRIORITY_ENDPOINT=https://api.example.com/priority-workers

# Refresh interval in seconds (default: 900 = 15 minutes)
PRIORITY_REFRESH_INTERVAL_SECONDS=900
```

## External Endpoint Format

The external endpoint should return JSON in this format:

```json
{
  "workers": [
    "https://217.28.137.141:50035",
    "https://20.245.90.64:50001",
    "https://40.76.123.136:50042",
    "https://172.214.189.153:18080"
  ]
}
```

**Note**: The system currently uses the full URL as the worker ID. When submitting jobs, use the complete URL as the worker_id to match against the priority list.

## Job Flow

```
1. Job arrives from a submitter with their worker_id
2. System checks if the submitter's worker_id is in priority list
3. If priority submitter → Route to fast queue
4. If regular submitter → Route to slow queue
5. Tee-worker processes fast queue first, then slow queue
```

**Important**: The priority is based on the job submitter's worker ID, not the tee-worker's own ID. This allows certain job submitters to have their requests processed faster.

## API Endpoints

### Queue Statistics
```bash
GET /job/queue/stats
```

Response:
```json
{
  "fast_queue_depth": 10,
  "slow_queue_depth": 45,
  "fast_processed": 1234,
  "slow_processed": 5678,
  "last_update": "2024-01-15T10:30:00Z"
}
```

## Development & Testing

### Using Real Endpoint

To use the actual TEE workers endpoint:
```bash
export EXTERNAL_WORKER_ID_PRIORITY_ENDPOINT="https://tee-api.masa.ai/list-tee-workers"
```

### Using Dummy Data

When no external endpoint is configured or if the endpoint fails, the system falls back to dummy priority worker IDs:
- `worker-001`, `worker-002`, `worker-005`
- `worker-priority-1`, `worker-priority-2`
- `worker-vip-1`
- `worker-high-priority-3`
- `worker-fast-lane-1`

### Disable Priority Queue

To run in legacy mode (single queue):
```bash
ENABLE_PRIORITY_QUEUE=false
```

## Implementation Details

### Files Added/Modified

1. **New Files**:
   - `internal/jobserver/priority_queue.go` - Dual queue implementation
   - `internal/jobserver/priority_manager.go` - Priority worker list management
   - `internal/jobserver/errors.go` - Error definitions

2. **Modified Files**:
   - `internal/jobserver/jobserver.go` - Integration with priority system
   - `internal/jobserver/worker.go` - Priority-aware job processing
   - `api/types/job.go` - Added GetBool helper method
   - `internal/api/routes.go` - Added queue stats endpoint
   - `internal/api/start.go` - Registered new endpoint

### Key Features

- **Non-breaking**: Falls back to legacy mode when disabled
- **Resilient**: Uses dummy data if external endpoint fails
- **Observable**: Queue statistics endpoint for monitoring
- **Configurable**: All parameters can be tuned via environment
- **Concurrent**: Thread-safe operations with proper locking

## Example Usage

### Start with Priority Queue
```bash
export ENABLE_PRIORITY_QUEUE=true
export EXTERNAL_WORKER_ID_PRIORITY_ENDPOINT="https://your-api.com/priority-workers"
export FAST_QUEUE_SIZE=2000
export SLOW_QUEUE_SIZE=10000
export PRIORITY_REFRESH_INTERVAL_SECONDS=300  # 5 minutes

./tee-worker
```

### Monitor Queue Performance
```bash
# Check queue statistics
curl http://localhost:8080/job/queue/stats

# Response shows queue depths and processing counts
{
  "fast_queue_depth": 5,
  "slow_queue_depth": 23,
  "fast_processed": 1523,
  "slow_processed": 4821,
  "last_update": "2024-01-15T14:22:31Z"
}
```

## Endpoint Integration Details

### Automatic Refresh
The priority list is automatically refreshed from the external endpoint:
- Initial fetch on startup
- Periodic refresh every 15 minutes (configurable)
- Continues using last known good list if refresh fails
- All errors are logged but don't stop the service

### Monitoring Endpoint Status
Check logs for endpoint status:
```
INFO[0000] Fetching initial priority list from external endpoint: https://tee-api.masa.ai/list-tee-workers
INFO[0000] Priority list updated with 179 workers from external endpoint
```

## Future Enhancements

1. **Dynamic Queue Sizing**: Adjust queue sizes based on load
2. **Priority Levels**: Multiple priority tiers (not just fast/slow)
3. **Metrics Export**: Prometheus/Grafana integration
4. **Queue Persistence**: Survive restarts without losing jobs
5. **Worker ID Extraction**: Extract worker ID from URL if needed (currently uses full URL)