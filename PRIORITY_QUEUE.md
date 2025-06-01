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
  "worker_ids": [
    "worker-001",
    "worker-002",
    "worker-priority-1",
    "worker-vip-1"
  ],
  "updated_at": "2024-01-15T10:00:00Z"
}
```

## Job Flow

```
1. Job arrives with worker_id
2. System checks if worker_id is in priority list
3. If priority worker → Route to fast queue
4. If regular worker → Route to slow queue
5. Workers process fast queue first, then slow queue
```

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

### Using Dummy Data

When no external endpoint is configured, the system uses these dummy priority worker IDs:
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

## Future Enhancements

1. **Dynamic Queue Sizing**: Adjust queue sizes based on load
2. **Priority Levels**: Multiple priority tiers (not just fast/slow)
3. **Metrics Export**: Prometheus/Grafana integration
4. **Queue Persistence**: Survive restarts without losing jobs
5. **Real External Endpoint**: Replace dummy implementation with actual HTTP calls