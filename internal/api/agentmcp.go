package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	samplev1 "github.com/adiom-data/crew-demo-app/gen/go/sample/v1"
	"github.com/adiom-data/grpcmcp/grpcmcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// agentMCPServiceName is the only service exposed as MCP tools. It must stay a
// read-only, side-effect-free surface — the /mcp endpoint is unauthenticated.
const agentMCPServiceName protoreflect.FullName = "sample.v1.AgentQueryService"

// agentMCPHandler serves the streamable-HTTP MCP endpoint. It builds the grpcmcp server
// lazily on the first request and caches it. grpcmcp proxies the app's own Connect
// backend (self-dialed at BaseURL); tool calls are plain unary POSTs.
//
// Descriptors come from the compiled-in generated protos rather than gRPC reflection:
// reflection is a streaming RPC, and the framework's request-logging middleware wraps
// the ResponseWriter in a recorder that does not implement http.Flusher, which breaks
// streaming responses on the self-dial.
type agentMCPHandler struct {
	selfBaseURL string

	mu      sync.Mutex
	handler http.Handler
}

func newAgentMCPHandler(selfBaseURL string) *agentMCPHandler {
	return &agentMCPHandler{selfBaseURL: selfBaseURL}
}

func (h *agentMCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, err := h.ensure()
	if err != nil {
		slog.Warn("agent mcp endpoint unavailable", "err", err)
		http.Error(w, "agent mcp unavailable", http.StatusServiceUnavailable)
		return
	}
	handler.ServeHTTP(w, r)
}

func (h *agentMCPHandler) ensure() (http.Handler, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.handler != nil {
		return h.handler, nil
	}
	srv, err := grpcmcp.NewServer(grpcmcp.Config{
		ServerName:  "crew-demo-app",
		Version:     "1.0.0",
		BaseURL:     h.selfBaseURL,
		UseConnect:  true,
		Descriptors: agentMCPDescriptors(),
		Services:    []protoreflect.FullName{agentMCPServiceName},
	})
	if err != nil {
		return nil, fmt.Errorf("build mcp server: %w", err)
	}
	h.handler = mcpserver.NewStreamableHTTPServer(srv, mcpserver.WithStateLess(true))
	return h.handler, nil
}

// agentMCPDescriptors flattens the AgentQueryService file descriptor plus all transitive
// imports into a FileDescriptorSet, dependencies first — the order grpcmcp.NewServer
// needs to protodesc.NewFile each entry against a fresh registry.
func agentMCPDescriptors() *descriptorpb.FileDescriptorSet {
	out := &descriptorpb.FileDescriptorSet{}
	seen := map[string]bool{}
	var add func(fd protoreflect.FileDescriptor)
	add = func(fd protoreflect.FileDescriptor) {
		if seen[fd.Path()] {
			return
		}
		seen[fd.Path()] = true
		imports := fd.Imports()
		for i := 0; i < imports.Len(); i++ {
			add(imports.Get(i).FileDescriptor) // post-order: deps precede dependents
		}
		out.File = append(out.File, protodesc.ToFileDescriptorProto(fd))
	}
	add(samplev1.File_sample_v1_agentquery_proto)
	return out
}
