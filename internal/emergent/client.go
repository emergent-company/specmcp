// Package emergent provides a domain-specific wrapper around the Emergent SDK.
// It exposes typed operations for SpecMCP entity types rather than generic graph operations.
package emergent

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
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
	sdk    *sdk.Client
	logger *slog.Logger
}

// ClientFactory creates per-request Emergent clients. It holds the shared
// configuration (server URL) and a shared http.Client for connection pooling.
// Each request gets its own SDK client with the auth token from context.
//
// This enables a multi-tenant architecture where different MCP clients can
// connect with different Emergent project tokens through the same SpecMCP server.
type ClientFactory struct {
	serverURL  string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClientFactory creates a factory for per-request Emergent clients.
// The shared http.Client reuses TCP connections across requests.
func NewClientFactory(serverURL string, logger *slog.Logger) *ClientFactory {
	return &ClientFactory{
		serverURL: serverURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// ClientFor creates an Emergent client using the auth token from the context.
// Each call creates a lightweight SDK client (~28 allocations, zero I/O) that
// shares the factory's connection pool. Returns an error if no token is in context.
func (f *ClientFactory) ClientFor(ctx context.Context) (*Client, error) {
	token := TokenFrom(ctx)
	if token == "" {
		return nil, fmt.Errorf("no emergent token in request context")
	}

	sdkClient, err := sdk.New(sdk.Config{
		ServerURL: f.serverURL,
		Auth: sdk.AuthConfig{
			Mode:   "apikey",
			APIKey: token,
		},
		HTTPClient: f.httpClient,
	})
	if err != nil {
		return nil, fmt.Errorf("creating SDK client: %w", err)
	}

	return &Client{
		sdk:    sdkClient,
		logger: f.logger,
	}, nil
}

// NewClient creates a Client with a fixed auth token. Use this for CLI tools
// (like the seed script) that operate with a single known token rather than
// per-request tokens from HTTP headers.
func NewClient(serverURL, token string, logger *slog.Logger) (*Client, error) {
	sdkClient, err := sdk.New(sdk.Config{
		ServerURL: serverURL,
		Auth: sdk.AuthConfig{
			Mode:   "apikey",
			APIKey: token,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating SDK client: %w", err)
	}
	return &Client{
		sdk:    sdkClient,
		logger: logger,
	}, nil
}

// Graph returns the underlying SDK graph client for advanced operations.
func (c *Client) Graph() *graph.Client {
	return c.sdk.Graph
}

// CreateObject creates a graph object with the given type, key, properties, and labels.
func (c *Client) CreateObject(ctx context.Context, typeName string, key *string, props map[string]any, labels []string) (*graph.GraphObject, error) {
	obj, err := c.sdk.Graph.CreateObject(ctx, &graph.CreateObjectRequest{
		Type:       typeName,
		Key:        key,
		Properties: props,
		Labels:     labels,
	})
	if err != nil {
		return nil, fmt.Errorf("creating %s object: %w", typeName, err)
	}
	c.logger.Debug("created object", "type", typeName, "id", obj.ID, "key", key)
	return obj, nil
}

// GetObject retrieves a graph object by ID.
func (c *Client) GetObject(ctx context.Context, id string) (*graph.GraphObject, error) {
	obj, err := c.sdk.Graph.GetObject(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting object %s: %w", id, err)
	}
	return obj, nil
}

// GetObjects retrieves multiple graph objects by their IDs in a single request.
func (c *Client) GetObjects(ctx context.Context, ids []string) ([]*graph.GraphObject, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	objs, err := c.sdk.Graph.GetObjects(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("getting objects: %w", err)
	}
	return objs, nil
}

// UpdateObject updates a graph object's properties and/or labels.
func (c *Client) UpdateObject(ctx context.Context, id string, props map[string]any, labels []string) (*graph.GraphObject, error) {
	req := &graph.UpdateObjectRequest{
		Properties: props,
	}
	if labels != nil {
		req.Labels = labels
		replaceLabels := true
		req.ReplaceLabels = &replaceLabels
	}
	obj, err := c.sdk.Graph.UpdateObject(ctx, id, req)
	if err != nil {
		return nil, fmt.Errorf("updating object %s: %w", id, err)
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
	resp, err := c.sdk.Graph.ListObjects(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("listing objects: %w", err)
	}
	return resp.Items, nil
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
	rel, err := c.sdk.Graph.CreateRelationship(ctx, &graph.CreateRelationshipRequest{
		Type:       relType,
		SrcID:      srcID,
		DstID:      dstID,
		Properties: props,
	})
	if err != nil {
		return nil, fmt.Errorf("creating %s relationship: %w", relType, err)
	}
	c.logger.Debug("created relationship", "type", relType, "src", srcID, "dst", dstID, "id", rel.ID)
	return rel, nil
}

// ListRelationships lists relationships with filtering options.
func (c *Client) ListRelationships(ctx context.Context, opts *graph.ListRelationshipsOptions) ([]*graph.GraphRelationship, error) {
	resp, err := c.sdk.Graph.ListRelationships(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("listing relationships: %w", err)
	}
	return resp.Items, nil
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
	resp, err := c.sdk.Graph.ExpandGraph(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("expanding graph: %w", err)
	}
	return resp, nil
}

// FTSSearch performs a full-text search across graph objects.
func (c *Client) FTSSearch(ctx context.Context, opts *graph.FTSSearchOptions) (*graph.SearchResponse, error) {
	resp, err := c.sdk.Graph.FTSSearch(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("FTS search: %w", err)
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
