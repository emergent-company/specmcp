// Package emergent provides a domain-specific wrapper around the Emergent SDK.
// It exposes typed operations for SpecMCP entity types rather than generic graph operations.
package emergent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	sdk "github.com/emergent-company/emergent/apps/server-go/pkg/sdk"
	"github.com/emergent-company/emergent/apps/server-go/pkg/sdk/graph"
)

// contextKey is an unexported type for context keys in this package.
type contextKey struct{}

// tokenKey is the context key for the Emergent auth token.
var tokenKey = contextKey{}

// WithToken returns a context carrying the given Emergent auth token.
// The token is used by ClientFactory.ClientFor to create per-request SDK clients.
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

// TokenFrom extracts the Emergent auth token from the context.
// Returns empty string if no token is present.
func TokenFrom(ctx context.Context) string {
	if v, ok := ctx.Value(tokenKey).(string); ok {
		return v
	}
	return ""
}

// Client wraps the Emergent SDK with domain-specific operations for SpecMCP.
type Client struct {
	sdk                    *sdk.Client
	logger                 *slog.Logger
	maxRetries             int // Maximum retry attempts for failed requests
	longOutageIntervalMins int // After many failures, switch to this interval in minutes
	longOutageThreshold    int // Number of consecutive failures before switching to long outage mode
}

// ClientFactory creates per-request Emergent clients. It holds the shared
// configuration (server URL) and a shared http.Client for connection pooling.
// Each request gets its own SDK client with the auth token from context.
//
// This enables a multi-tenant architecture where different MCP clients can
// connect with different Emergent project tokens through the same SpecMCP server.
//
// In HTTP mode, an optional adminToken can be provided as a fallback for
// server-side operations (like janitor) that don't have a user token in context.
type ClientFactory struct {
	serverURL              string
	adminToken             string // Optional: fallback token for server-side operations in HTTP mode
	httpClient             *http.Client
	logger                 *slog.Logger
	maxRetries             int // Maximum retry attempts for failed requests
	longOutageIntervalMins int // After many failures, switch to this interval in minutes
	longOutageThreshold    int // Number of consecutive failures before switching to long outage mode
}

// NewClientFactory creates a factory for per-request Emergent clients.
// The shared http.Client reuses TCP connections across requests.
// adminToken is optional and used as a fallback when no token is in the request context.
// maxRetries controls how many times to retry failed requests (0 = no retries, -1 = infinite).
// longOutageIntervalMins is the interval between retries after many consecutive failures.
// longOutageThreshold is the number of consecutive failures before switching to long outage mode.
func NewClientFactory(serverURL string, adminToken string, maxRetries int, longOutageIntervalMins int, longOutageThreshold int, logger *slog.Logger) *ClientFactory {
	// Configure HTTP transport with keep-alive and connection pooling
	transport := &http.Transport{
		// Connection pooling
		MaxIdleConns:        100,              // Maximum idle connections across all hosts
		MaxIdleConnsPerHost: 10,               // Maximum idle connections per host
		MaxConnsPerHost:     50,               // Maximum total connections per host
		IdleConnTimeout:     90 * time.Second, // How long idle connections stay in pool

		// Timeouts
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second, // Connection establishment timeout
			KeepAlive: 30 * time.Second, // TCP keep-alive interval
		}).DialContext,

		// TLS and other timeouts
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second, // Time to receive response headers
		ExpectContinueTimeout: 1 * time.Second,

		// Keep connections alive
		DisableKeepAlives: false,
		ForceAttemptHTTP2: true,
	}

	return &ClientFactory{
		serverURL:  serverURL,
		adminToken: adminToken,
		httpClient: &http.Client{
			Timeout:   5 * time.Minute, // Increased from 30s to 5 minutes for long operations
			Transport: transport,
		},
		logger:                 logger,
		maxRetries:             maxRetries,
		longOutageIntervalMins: longOutageIntervalMins,
		longOutageThreshold:    longOutageThreshold,
	}
}

// ClientFor creates an Emergent client using the auth token from the context.
// If no token is in context and adminToken is configured, uses the admin token.
// Each call creates a lightweight SDK client (~28 allocations, zero I/O) that
// shares the factory's connection pool. Returns an error if no token is available.
func (f *ClientFactory) ClientFor(ctx context.Context) (*Client, error) {
	token := TokenFrom(ctx)
	if token == "" {
		// Fallback to admin token for server-side operations (janitor, etc.)
		if f.adminToken != "" {
			token = f.adminToken
			f.logger.Debug("using admin token for server-side operation")
		} else {
			return nil, fmt.Errorf("no emergent token in request context and no admin token configured")
		}
	}

	// Check for project ID in environment for standalone API keys
	projectID := os.Getenv("EMERGENT_PROJECT_ID")

	sdkClient, err := sdk.New(sdk.Config{
		ServerURL: f.serverURL,
		Auth: sdk.AuthConfig{
			Mode:   "apikey",
			APIKey: token,
		},
		ProjectID:  projectID,
		HTTPClient: f.httpClient,
	})
	if err != nil {
		return nil, fmt.Errorf("creating SDK client: %w", err)
	}

	return &Client{
		sdk:                    sdkClient,
		logger:                 f.logger,
		maxRetries:             f.maxRetries,
		longOutageIntervalMins: f.longOutageIntervalMins,
		longOutageThreshold:    f.longOutageThreshold,
	}, nil
}

// NewClient creates a Client with a fixed auth token. Use this for CLI tools
// (like the seed script) that operate with a single known token rather than
// per-request tokens from HTTP headers.
func NewClient(serverURL, token string, logger *slog.Logger) (*Client, error) {
	// Check for project ID in environment for standalone API keys
	projectID := os.Getenv("EMERGENT_PROJECT_ID")

	sdkClient, err := sdk.New(sdk.Config{
		ServerURL: serverURL,
		Auth: sdk.AuthConfig{
			Mode:   "apikey",
			APIKey: token,
		},
		ProjectID: projectID,
	})
	if err != nil {
		return nil, fmt.Errorf("creating SDK client: %w", err)
	}
	return &Client{
		sdk:                    sdkClient,
		logger:                 logger,
		maxRetries:             5,  // Default for direct client creation
		longOutageIntervalMins: 5,  // Default to 5 minutes for long outages
		longOutageThreshold:    20, // Default to 20 consecutive failures
	}, nil
}

// Graph returns the underlying SDK graph client for advanced operations.
func (c *Client) Graph() *graph.Client {
	return c.sdk.Graph
}

// retryConfig holds retry behavior configuration.
type retryConfig struct {
	maxRetries          int
	initialBackoff      time.Duration
	maxBackoff          time.Duration
	backoffFactor       float64
	longOutageInterval  time.Duration
	longOutageThreshold int
}

// defaultRetryConfig returns the default retry configuration.
func (c *Client) getRetryConfig() retryConfig {
	return retryConfig{
		maxRetries:          c.maxRetries,
		initialBackoff:      500 * time.Millisecond,                                // Start fast
		maxBackoff:          1 * time.Minute,                                       // Cap at 1 minute for normal backoff
		backoffFactor:       2.0,                                                   // Exponential backoff
		longOutageInterval:  time.Duration(c.longOutageIntervalMins) * time.Minute, // Interval for long outages
		longOutageThreshold: c.longOutageThreshold,                                 // Switch to long outage mode after this many failures
	}
}

// shouldRetry determines if an error is retryable.
func shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Network errors are retryable
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Timeout errors are retryable
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Connection errors are retryable
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// EOF and connection reset errors are retryable
	errStr := err.Error()
	if errStr == "EOF" ||
		errStr == "unexpected EOF" ||
		errStr == "connection reset by peer" ||
		errStr == "broken pipe" {
		return true
	}

	return false
}

// withRetry wraps an operation with retry logic using exponential backoff.
// If maxRetries is -1, it will retry indefinitely (useful for maintaining persistent connections).
// After longOutageThreshold consecutive failures, switches to longOutageInterval for less aggressive retrying.
func (c *Client) withRetry(ctx context.Context, operation string, fn func() error) error {
	cfg := c.getRetryConfig()
	var lastErr error

	attempt := 0
	consecutiveFailures := 0
	for {
		// Check if we've exceeded max retries (unless it's -1 for infinite)
		if cfg.maxRetries >= 0 && attempt > cfg.maxRetries {
			break
		}

		if attempt > 0 {
			// Determine if we're in "long outage mode"
			inLongOutageMode := consecutiveFailures >= cfg.longOutageThreshold

			var backoff time.Duration
			if inLongOutageMode {
				// Use configured long outage interval
				backoff = cfg.longOutageInterval
				c.logger.Warn("retrying operation in long outage mode",
					"operation", operation,
					"attempt", attempt,
					"consecutive_failures", consecutiveFailures,
					"backoff", backoff,
					"error", lastErr,
				)
			} else {
				// Calculate exponential backoff
				multiplier := 1 << uint(attempt-1) // 1, 2, 4, 8, 16...
				backoff = cfg.initialBackoff * time.Duration(multiplier)
				if backoff > cfg.maxBackoff {
					backoff = cfg.maxBackoff
				}

				c.logger.Warn("retrying operation after error",
					"operation", operation,
					"attempt", attempt,
					"max_retries", cfg.maxRetries,
					"backoff", backoff,
					"error", lastErr,
				)
			}

			select {
			case <-time.After(backoff):
				// Continue with retry
			case <-ctx.Done():
				return fmt.Errorf("%s: context cancelled during retry: %w", operation, ctx.Err())
			}
		}

		err := fn()
		if err == nil {
			if attempt > 0 {
				c.logger.Info("operation succeeded after retry",
					"operation", operation,
					"attempts", attempt+1,
					"consecutive_failures", consecutiveFailures,
				)
			}
			return nil
		}

		lastErr = err

		// Don't retry if error is not retryable
		if !shouldRetry(err) {
			return fmt.Errorf("%s: %w", operation, err)
		}

		// Increment counters
		attempt++
		consecutiveFailures++

		// Log if we're in infinite retry mode and hitting milestones
		if cfg.maxRetries < 0 {
			if consecutiveFailures == cfg.longOutageThreshold {
				c.logger.Warn("switching to long outage mode",
					"operation", operation,
					"consecutive_failures", consecutiveFailures,
					"new_interval", cfg.longOutageInterval,
				)
			}
			if consecutiveFailures%10 == 0 {
				c.logger.Warn("still retrying operation in infinite mode",
					"operation", operation,
					"attempts", attempt,
					"consecutive_failures", consecutiveFailures,
					"last_error", lastErr,
				)
			}
		}
	}

	return fmt.Errorf("%s: failed after %d attempts: %w", operation, cfg.maxRetries+1, lastErr)
}

// CreateObject creates a graph object with the given type, key, properties, and labels.
func (c *Client) CreateObject(ctx context.Context, typeName string, key *string, props map[string]any, labels []string) (*graph.GraphObject, error) {
	var obj *graph.GraphObject
	err := c.withRetry(ctx, fmt.Sprintf("create %s object", typeName), func() error {
		var createErr error
		obj, createErr = c.sdk.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
			Type:       typeName,
			Key:        key,
			Properties: props,
			Labels:     labels,
		})
		return createErr
	})
	if err != nil {
		return nil, err
	}
	c.logger.Debug("created object", "type", typeName, "id", obj.ID, "key", key)
	return obj, nil
}

// GetObject retrieves a graph object by ID.
func (c *Client) GetObject(ctx context.Context, id string) (*graph.GraphObject, error) {
	var obj *graph.GraphObject
	err := c.withRetry(ctx, fmt.Sprintf("get object %s", id), func() error {
		var getErr error
		obj, getErr = c.sdk.Graph.GetObject(ctx, id)
		return getErr
	})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// GetObjects retrieves multiple graph objects by their IDs in a single request.
func (c *Client) GetObjects(ctx context.Context, ids []string) ([]*graph.GraphObject, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var objs []*graph.GraphObject
	err := c.withRetry(ctx, "get objects batch", func() error {
		var getErr error
		objs, getErr = c.sdk.Graph.GetObjects(ctx, ids)
		return getErr
	})
	if err != nil {
		return nil, err
	}
	return objs, nil
}

// UpdateObject updates a graph object's properties and/or labels.
func (c *Client) UpdateObject(ctx context.Context, id string, props map[string]any, labels []string) (*graph.GraphObject, error) {
	var obj *graph.GraphObject
	err := c.withRetry(ctx, fmt.Sprintf("update object %s", id), func() error {
		req := &graph.UpdateObjectRequest{
			Properties: props,
		}
		if labels != nil {
			req.Labels = labels
			replaceLabels := true
			req.ReplaceLabels = &replaceLabels
		}
		var updateErr error
		obj, updateErr = c.sdk.Graph.UpdateObject(ctx, id, req)
		return updateErr
	})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// DeleteObject soft-deletes a graph object.
func (c *Client) DeleteObject(ctx context.Context, id string) error {
	if err := c.sdk.Graph.DeleteObject(ctx, id); err != nil {
		return fmt.Errorf("deleting object %s: %w", id, err)
	}
	return nil
}

// ListObjects lists objects with filtering options.
func (c *Client) ListObjects(ctx context.Context, opts *graph.ListObjectsOptions) ([]*graph.GraphObject, error) {
	var items []*graph.GraphObject
	err := c.withRetry(ctx, "list objects", func() error {
		resp, listErr := c.sdk.Graph.ListObjects(ctx, opts)
		if listErr != nil {
			return listErr
		}
		items = resp.Items
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// CountObjects returns the total count of objects matching the given type and filters.
// Uses the native SDK CountObjects endpoint (server-side count, no data transfer).
func (c *Client) CountObjects(ctx context.Context, typeName string) (int, error) {
	count, err := c.sdk.Graph.CountObjects(ctx, &graph.CountObjectsOptions{
		Type: typeName,
	})
	if err != nil {
		return 0, fmt.Errorf("counting objects: %w", err)
	}
	return count, nil
}

// UpsertObject creates or updates a graph object by (type, key).
// If an object with the same type and key exists, it is updated; otherwise created.
func (c *Client) UpsertObject(ctx context.Context, typeName string, key *string, props map[string]any, labels []string) (*graph.GraphObject, error) {
	obj, err := c.sdk.Graph.UpsertObject(ctx, &graph.CreateObjectRequest{
		Type:       typeName,
		Key:        key,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return nil, fmt.Errorf("upserting %s object: %w", typeName, err)
	}
	c.logger.Debug("upserted object", "type", typeName, "id", obj.ID, "key", key)
	return obj, nil
}

// FindByTypeAndKey finds a single object by type and key.
// Returns nil, nil if not found.
// When multiple objects share the same type+key (duplicates from before dedup was added),
// this returns the one with the smallest ID (string sort) for determinism.
func (c *Client) FindByTypeAndKey(ctx context.Context, typeName, key string) (*graph.GraphObject, error) {
	items, err := c.ListObjects(ctx, &graph.ListObjectsOptions{
		Type:  typeName,
		Key:   key,
		Limit: 50, // Fetch enough to find all duplicates
	})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	if len(items) > 1 {
		c.logger.Warn("found multiple objects with same type+key", "type", typeName, "key", key, "count", len(items))
	}
	// Pick the one with the smallest CanonicalID for determinism across versions
	oldest := items[0]
	for _, item := range items[1:] {
		if item.CanonicalID < oldest.CanonicalID {
			oldest = item
		}
	}
	return oldest, nil
}

// CreateRelationship creates a relationship between two objects.
func (c *Client) CreateRelationship(ctx context.Context, relType, srcID, dstID string, props map[string]any) (*graph.GraphRelationship, error) {
	var rel *graph.GraphRelationship
	err := c.withRetry(ctx, fmt.Sprintf("create %s relationship", relType), func() error {
		var createErr error
		rel, createErr = c.sdk.Graph.CreateRelationship(ctx, &graph.CreateRelationshipRequest{
			Type:       relType,
			SrcID:      srcID,
			DstID:      dstID,
			Properties: props,
		})
		return createErr
	})
	if err != nil {
		return nil, err
	}
	c.logger.Debug("created relationship", "type", relType, "src", srcID, "dst", dstID, "id", rel.ID)
	return rel, nil
}

// ListRelationships lists relationships with filtering options.
func (c *Client) ListRelationships(ctx context.Context, opts *graph.ListRelationshipsOptions) ([]*graph.GraphRelationship, error) {
	var items []*graph.GraphRelationship
	err := c.withRetry(ctx, "list relationships", func() error {
		resp, listErr := c.sdk.Graph.ListRelationships(ctx, opts)
		if listErr != nil {
			return listErr
		}
		items = resp.Items
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// GetObjectEdges returns all incoming and outgoing relationships for an object.
// Pass nil for opts to get all edges without filtering; use GetObjectEdgesOptions
// to filter by type or direction.
func (c *Client) GetObjectEdges(ctx context.Context, objectID string, opts *graph.GetObjectEdgesOptions) (*graph.GetObjectEdgesResponse, error) {
	edges, err := c.sdk.Graph.GetObjectEdges(ctx, objectID, opts)
	if err != nil {
		return nil, fmt.Errorf("getting edges for %s: %w", objectID, err)
	}
	return edges, nil
}

// ExpandGraph performs a graph expansion from root nodes.
func (c *Client) ExpandGraph(ctx context.Context, req *graph.GraphExpandRequest) (*graph.GraphExpandResponse, error) {
	var resp *graph.GraphExpandResponse
	err := c.withRetry(ctx, "expand graph", func() error {
		var expandErr error
		resp, expandErr = c.sdk.Graph.ExpandGraph(ctx, req)
		return expandErr
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// FTSSearch performs a full-text search across graph objects.
func (c *Client) FTSSearch(ctx context.Context, opts *graph.FTSSearchOptions) (*graph.SearchResponse, error) {
	var resp *graph.SearchResponse
	err := c.withRetry(ctx, "FTS search", func() error {
		var searchErr error
		resp, searchErr = c.sdk.Graph.FTSSearch(ctx, opts)
		return searchErr
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// DeleteRelationship soft-deletes a relationship.
func (c *Client) DeleteRelationship(ctx context.Context, id string) error {
	if err := c.sdk.Graph.DeleteRelationship(ctx, id); err != nil {
		return fmt.Errorf("deleting relationship %s: %w", id, err)
	}
	return nil
}
