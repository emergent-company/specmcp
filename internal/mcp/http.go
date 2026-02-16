// Package mcp provides the MCP protocol server implementation.
// This file implements the Streamable HTTP transport per MCP spec 2025-03-26.
package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/emergent-company/specmcp/internal/emergent"
)

// HTTPServer wraps Server with Streamable HTTP transport (MCP spec 2025-03-26).
// It serves a single MCP endpoint that accepts POST (JSON-RPC messages) and
// GET (SSE stream for server-initiated messages).
//
// Authentication: clients must send their Emergent project token as a Bearer
// token in the Authorization header. This token is injected into the request
// context and used by ClientFactory.ClientFor to create per-request SDK clients.
type HTTPServer struct {
	server   *Server
	cors     string
	logger   *slog.Logger
	sessions sync.Map // sessionID -> *session
}

// session tracks an MCP session established via initialize.
type session struct {
	id        string
	createdAt time.Time
}

// NewHTTPServer creates an HTTP transport wrapper around the core MCP server.
func NewHTTPServer(server *Server, corsOrigins string, logger *slog.Logger) *HTTPServer {
	return &HTTPServer{
		server: server,
		cors:   corsOrigins,
		logger: logger,
	}
}

// Handler returns an http.Handler that serves the MCP Streamable HTTP endpoint.
// Mount this at your desired path (e.g. "/mcp").
func (h *HTTPServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", h.handleMCP)
	// Health check endpoint for deployment probes.
	mux.HandleFunc("/health", h.handleHealth)
	return mux
}

// handleHealth responds to health check probes.
func (h *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleMCP is the single MCP endpoint that supports POST and GET.
func (h *HTTPServer) handleMCP(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers on every response.
	h.setCORS(w, r)

	// Handle CORS preflight.
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Authenticate all requests except OPTIONS.
	if !h.authenticate(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handlePost(w, h.injectToken(r))
	case http.MethodGet:
		h.handleGet(w, h.injectToken(r))
	case http.MethodDelete:
		h.handleDelete(w, r)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE, OPTIONS")
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handlePost processes JSON-RPC messages from the client.
func (h *HTTPServer) handlePost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		http.Error(w, `{"error":"failed to read request body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		http.Error(w, `{"error":"empty request body"}`, http.StatusBadRequest)
		return
	}

	// Determine if this is a batch or single message.
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "[") {
		h.handleBatch(w, r, body)
		return
	}

	h.handleSingle(w, r, body)
}

// handleSingle processes a single JSON-RPC message.
func (h *HTTPServer) handleSingle(w http.ResponseWriter, r *http.Request, body []byte) {
	// Peek at the message to check if it's a notification or response (no ID).
	var peek struct {
		ID     json.RawMessage `json:"id,omitempty"`
		Method string          `json:"method,omitempty"`
	}
	if err := json.Unmarshal(body, &peek); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, ErrCodeParse, "Parse error", err.Error())
		return
	}

	// Notifications and responses: accept with 202.
	isNotification := peek.ID == nil || string(peek.ID) == "null"
	if isNotification {
		// Still process it (e.g. notifications/initialized).
		_ = h.server.HandleMessage(r.Context(), body)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// It's a request — process and respond.
	resp := h.server.HandleMessage(r.Context(), body)
	if resp == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Check if this is an initialize response — if so, create a session.
	if peek.Method == "initialize" && resp.Error == nil {
		sessionID := h.createSession()
		w.Header().Set("Mcp-Session-Id", sessionID)
	}

	// Validate session for non-initialize requests.
	if peek.Method != "initialize" {
		sessionID := r.Header.Get("Mcp-Session-Id")
		if sessionID != "" {
			if _, ok := h.sessions.Load(sessionID); !ok {
				http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
				return
			}
		}
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// handleBatch processes a JSON-RPC batch.
func (h *HTTPServer) handleBatch(w http.ResponseWriter, r *http.Request, body []byte) {
	var messages []json.RawMessage
	if err := json.Unmarshal(body, &messages); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, ErrCodeParse, "Parse error", err.Error())
		return
	}

	if len(messages) == 0 {
		h.writeJSONError(w, http.StatusBadRequest, ErrCodeInvalidRequest, "Empty batch", nil)
		return
	}

	// Process each message, collect responses.
	var responses []*Response
	allNotifications := true

	for _, msg := range messages {
		var peek struct {
			ID json.RawMessage `json:"id,omitempty"`
		}
		if err := json.Unmarshal(msg, &peek); err != nil {
			continue
		}

		isNotification := peek.ID == nil || string(peek.ID) == "null"
		if !isNotification {
			allNotifications = false
		}

		resp := h.server.HandleMessage(r.Context(), msg)
		if resp != nil {
			responses = append(responses, resp)
		}
	}

	if allNotifications || len(responses) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	h.writeJSON(w, http.StatusOK, responses)
}

// handleGet opens an SSE stream for server-initiated messages.
// For now, we return 405 since the server doesn't send unsolicited messages.
func (h *HTTPServer) handleGet(w http.ResponseWriter, r *http.Request) {
	accept := r.Header.Get("Accept")
	if !strings.Contains(accept, "text/event-stream") {
		http.Error(w, `{"error":"Accept header must include text/event-stream"}`, http.StatusBadRequest)
		return
	}

	// Per MCP spec: server MAY return 405 if it doesn't offer an SSE stream.
	// This server currently has no server-initiated messages.
	w.Header().Set("Allow", "POST, DELETE, OPTIONS")
	http.Error(w, `{"error":"SSE stream not supported; use POST for requests"}`, http.StatusMethodNotAllowed)
}

// handleDelete terminates a session.
func (h *HTTPServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		http.Error(w, `{"error":"Mcp-Session-Id header required"}`, http.StatusBadRequest)
		return
	}

	if _, ok := h.sessions.LoadAndDelete(sessionID); !ok {
		http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
		return
	}

	h.logger.Info("session terminated", "session_id", sessionID)
	w.WriteHeader(http.StatusOK)
}

// authenticate checks that the request contains an Authorization header with
// a Bearer token. The token itself is not validated here — it is passed through
// to the Emergent API which handles authentication. This ensures every request
// carries a token for ClientFactory.ClientFor to use.
func (h *HTTPServer) authenticate(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false
	}

	const bearerPrefix = "Bearer "
	if strings.HasPrefix(auth, bearerPrefix) {
		return strings.TrimPrefix(auth, bearerPrefix) != ""
	}

	// Non-bearer auth is not supported.
	return false
}

// injectToken extracts the bearer token from the Authorization header and
// injects it into the request context as the Emergent auth token. In HTTP mode,
// the bearer token serves as the Emergent project token, enabling per-request
// client creation via ClientFactory.ClientFor.
func (h *HTTPServer) injectToken(r *http.Request) *http.Request {
	auth := r.Header.Get("Authorization")
	token := strings.TrimPrefix(auth, "Bearer ")
	if token != "" {
		ctx := emergent.WithToken(r.Context(), token)
		return r.WithContext(ctx)
	}
	return r
}

// createSession generates a new session ID and stores it.
func (h *HTTPServer) createSession() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID (should never happen in practice).
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	id := hex.EncodeToString(b)
	h.sessions.Store(id, &session{
		id:        id,
		createdAt: time.Now(),
	})
	h.logger.Info("session created", "session_id", id)
	return id
}

// setCORS sets CORS headers on the response.
func (h *HTTPServer) setCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}

	if h.cors == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		// Check if the origin is in the allowed list.
		allowed := strings.Split(h.cors, ",")
		for _, a := range allowed {
			if strings.TrimSpace(a) == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}
	}

	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, Mcp-Session-Id")
	w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
}

// writeJSON writes a JSON response with the given status code.
func (h *HTTPServer) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.logger.Error("failed to write JSON response", "error", err)
	}
}

// writeJSONError writes a JSON-RPC error response.
func (h *HTTPServer) writeJSONError(w http.ResponseWriter, httpStatus int, code int, message string, data any) {
	resp := &Response{
		JSONRPC: "2.0",
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	h.writeJSON(w, httpStatus, resp)
}
