package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/adiom-data/grpcmcp/grpcmcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// agentMCPServiceName is the only service exposed as MCP tools. It must stay a
// read-only, side-effect-free surface — the /mcp endpoint is unauthenticated.
const agentMCPServiceName protoreflect.FullName = "sample.v1.AgentQueryService"

// agentMCPHandler serves the streamable-HTTP MCP endpoint. It builds the grpcmcp server
// lazily on the first request: the endpoint proxies the app's own Connect backend, which
// is not yet listening while Run() assembles routes, so descriptors are loaded via gRPC
// reflection against the self URL at first use (and retried if that fails).
type agentMCPHandler struct {
	selfBaseURL string

	mu      sync.Mutex
	handler http.Handler
}

func newAgentMCPHandler(selfBaseURL string) *agentMCPHandler {
	return &agentMCPHandler{selfBaseURL: selfBaseURL}
}

func (h *agentMCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, err := h.ensure(r.Context())
	if err != nil {
		slog.Warn("agent mcp endpoint unavailable", "err", err)
		http.Error(w, "agent mcp unavailable", http.StatusServiceUnavailable)
		return
	}
	handler.ServeHTTP(w, r)
}

func (h *agentMCPHandler) ensure(ctx context.Context) (http.Handler, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.handler != nil {
		return h.handler, nil
	}
	descriptors, err := grpcmcp.LoadDescriptorsFromReflection(ctx, h.selfBaseURL, nil, true)
	if err != nil {
		return nil, fmt.Errorf("load descriptors via reflection: %w", err)
	}
	srv, err := grpcmcp.NewServer(grpcmcp.Config{
		ServerName:  "crew-demo-app",
		Version:     "1.0.0",
		BaseURL:     h.selfBaseURL,
		UseConnect:  true,
		Descriptors: descriptors,
		Services:    []protoreflect.FullName{agentMCPServiceName},
	})
	if err != nil {
		return nil, fmt.Errorf("build mcp server: %w", err)
	}
	h.handler = mcpserver.NewStreamableHTTPServer(srv, mcpserver.WithStateLess(true))
	return h.handler, nil
}
