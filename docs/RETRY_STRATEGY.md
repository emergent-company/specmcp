# Automatic Retry Strategy

**Date**: 2026-02-17  
**Status**: Implemented

## Overview

SpecMCP implements intelligent automatic retry logic to maintain reliable connections to Emergent, even in unstable network conditions. All Emergent API operations automatically retry on transient failures.

## Retry Configuration

### Default Behavior
- **Max Retries**: 5 attempts (configurable)
- **Initial Backoff**: 500ms
- **Max Backoff**: 1 minute
- **Strategy**: Exponential backoff

### Configurable Options

**Environment Variable**:
```bash
EMERGENT_MAX_RETRIES=5   # Default
EMERGENT_MAX_RETRIES=10  # More persistent
EMERGENT_MAX_RETRIES=-1  # Infinite (never give up)
```

**Config File**:
```toml
[emergent]
max_retries = 5  # or -1 for infinite
```

## Backoff Sequence

With default exponential backoff:

| Attempt | Backoff | Cumulative Time |
|---------|---------|-----------------|
| 1       | 0ms     | 0s              |
| 2       | 500ms   | 0.5s            |
| 3       | 1s      | 1.5s            |
| 4       | 2s      | 3.5s            |
| 5       | 4s      | 7.5s            |
| 6       | 8s      | 15.5s           |
| 7       | 16s     | 31.5s           |
| 8       | 32s     | 63.5s           |
| 9+      | 60s     | +60s per attempt|

**Total time before giving up (5 retries)**: ~7.5 seconds  
**Total time before giving up (10 retries)**: ~63 seconds

## Retryable Errors

The following errors trigger automatic retry:

### Network Errors
```go
net.Error           // General network errors
net.OpError         // Operation errors (connection refused, etc.)
context.DeadlineExceeded  // Timeout errors
```

### Connection Errors
```
EOF
unexpected EOF
connection reset by peer
broken pipe
connection refused
network is unreachable
```

## Non-Retryable Errors

These errors fail immediately (no retry):

### Application Errors
```
validation error: name required
object not found
permission denied
invalid request
```

### HTTP Status Codes
```
400 Bad Request
401 Unauthorized  
403 Forbidden
404 Not Found
422 Unprocessable Entity
```

## Operations with Retry

All Emergent API operations have automatic retry:

| Operation Category | Operations |
|-------------------|------------|
| **Objects** | CreateObject, GetObject, GetObjects, UpdateObject, DeleteObject, ListObjects, CountObjects, UpsertObject |
| **Relationships** | CreateRelationship, DeleteRelationship, ListRelationships, GetObjectEdges |
| **Graph** | ExpandGraph, FTSSearch |
| **Queries** | FindByTypeAndKey |

## Usage Examples

### Example 1: Default Retry (5 Attempts)

```bash
# No configuration needed - works out of the box
EMERGENT_TOKEN=emt_... ./specmcp
```

**Log Output**:
```
INFO  calling tool tool=spec_new
WARN  retrying operation after error operation="create Change object" attempt=1 backoff=500ms error="dial tcp: connection refused"
WARN  retrying operation after error operation="create Change object" attempt=2 backoff=1s error="dial tcp: connection refused"
INFO  operation succeeded after retry operation="create Change object" attempts=3
INFO  tool execution completed tool=spec_new
```

### Example 2: Infinite Retry (Never Give Up)

```bash
# Perfect for production environments with occasional network issues
EMERGENT_MAX_RETRIES=-1 \
EMERGENT_TOKEN=emt_... \
./specmcp
```

**Log Output**:
```
WARN  retrying operation after error operation="expand graph" attempt=1 backoff=500ms
WARN  retrying operation after error operation="expand graph" attempt=2 backoff=1s
WARN  retrying operation after error operation="expand graph" attempt=3 backoff=2s
...
WARN  still retrying operation in infinite mode operation="expand graph" attempts=10
...
WARN  still retrying operation in infinite mode operation="expand graph" attempts=20
INFO  operation succeeded after retry operation="expand graph" attempts=23
```

### Example 3: Aggressive Retry (10 Attempts)

```bash
# For unstable networks - more attempts before giving up
EMERGENT_MAX_RETRIES=10 \
EMERGENT_TOKEN=emt_... \
./specmcp
```

### Example 4: Fast Fail (1 Attempt)

```bash
# For testing or high-throughput scenarios
EMERGENT_MAX_RETRIES=0 \
EMERGENT_TOKEN=emt_... \
./specmcp
```

## Monitoring Retry Behavior

### Enable Debug Logging

```bash
SPECMCP_LOG_LEVEL=debug \
EMERGENT_MAX_RETRIES=5 \
./specmcp
```

### Watch for Retry Patterns

```bash
# Filter logs for retry activity
./specmcp 2>&1 | grep "retrying operation"
./specmcp 2>&1 | grep "operation succeeded after retry"
```

### Key Log Messages

| Message | Meaning |
|---------|---------|
| `retrying operation after error` | Operation failed, retrying with backoff |
| `operation succeeded after retry` | Retry was successful |
| `still retrying operation in infinite mode` | Logged every 10 attempts in infinite mode |
| `context cancelled during retry` | User/system cancelled during backoff |
| `failed after N attempts` | All retries exhausted (max retries reached) |

## Best Practices

### Production Deployments

**Recommended Configuration**:
```toml
[emergent]
max_retries = -1  # Never give up - keep reconnecting

[transport]
request_timeout_minutes = 10  # Give retries time to work
idle_timeout_minutes = 10     # Keep connections alive
```

**Rationale**:
- Transient network issues resolve themselves
- Better to wait and retry than to fail permanently
- Users experience temporary slowdowns instead of errors

### Development Environments

**Recommended Configuration**:
```toml
[emergent]
max_retries = 3  # Fail faster to surface issues

[transport]
request_timeout_minutes = 5
idle_timeout_minutes = 5
```

**Rationale**:
- Faster feedback when Emergent is down
- Don't mask configuration issues
- Easier to debug

### Testing Retry Behavior

#### Test 1: Simulate Network Failure
```bash
# Terminal 1: Start SpecMCP with debug logging
SPECMCP_LOG_LEVEL=debug EMERGENT_MAX_RETRIES=10 ./specmcp

# Terminal 2: Stop Emergent
docker stop emergent

# Terminal 3: Try to create a change (will retry)
# (via MCP client)

# Terminal 2: Restart Emergent
docker start emergent

# Watch Terminal 1: Operation succeeds after retry!
```

#### Test 2: Verify Backoff Timing
```bash
# Start with debug logging
SPECMCP_LOG_LEVEL=debug EMERGENT_MAX_RETRIES=5 ./specmcp

# Stop Emergent to force retries
docker stop emergent

# Trigger operation and time the retries
time curl -X POST http://localhost:21452/mcp -d '...'

# Verify timing matches backoff sequence (0.5s + 1s + 2s + 4s + 8s)
```

## Implementation Details

### Code Location
- **Retry Logic**: `internal/emergent/client.go`
- **Configuration**: `internal/config/config.go`
- **Error Detection**: `shouldRetry()` function

### Key Functions

```go
// withRetry wraps any operation with automatic retry
func (c *Client) withRetry(ctx context.Context, operation string, fn func() error) error

// shouldRetry determines if an error is transient
func shouldRetry(err error) bool

// getRetryConfig gets the configured retry parameters
func (c *Client) getRetryConfig() retryConfig
```

### Example Usage in Code

```go
// All Emergent operations use withRetry internally
func (c *Client) CreateObject(...) (*graph.GraphObject, error) {
    var obj *graph.GraphObject
    err := c.withRetry(ctx, "create object", func() error {
        var createErr error
        obj, createErr = c.sdk.Graph.CreateObject(ctx, req)
        return createErr
    })
    return obj, err
}
```

## Troubleshooting

### Problem: Operations Timing Out

**Solution**: Increase retry count and timeout
```bash
EMERGENT_MAX_RETRIES=10
SPECMCP_REQUEST_TIMEOUT_MINUTES=10
```

### Problem: Too Many Retries Slowing Down System

**Solution**: Reduce retry count
```bash
EMERGENT_MAX_RETRIES=2
```

### Problem: Need to Know Why Operations Are Failing

**Solution**: Enable debug logging
```bash
SPECMCP_LOG_LEVEL=debug
```

### Problem: Network is Flaky But Eventually Recovers

**Solution**: Use infinite retries
```bash
EMERGENT_MAX_RETRIES=-1
```

## Performance Impact

### Minimal Overhead
- Retry logic only runs on failure
- Successful operations have no retry overhead
- Backoff uses efficient `time.After()` channels

### Resource Usage
- Each retry attempt uses same resources as original request
- Connection pooling minimizes connection establishment overhead
- Exponential backoff prevents server overload

### Latency
- **Success on first try**: No added latency
- **Success after retry**: Added latency = sum of backoff periods
- **Total failure**: Maximum latency = sum of all backoffs + request times

## See Also

- [Connection Improvements](./CONNECTION_IMPROVEMENTS.md) - Full connection management details
- [AGENTS.md](../AGENTS.md) - Technical architecture documentation
- [README.md](../README.md) - User-facing configuration guide
