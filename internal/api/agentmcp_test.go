package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	samplev1 "github.com/adiom-data/crew-demo-app/gen/go/sample/v1"
	"github.com/adiom-data/crew-demo-app/gen/go/sample/v1/samplev1connect"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func TestAgentQueryServiceListPartnersDBUnavailable(t *testing.T) {
	svc := agentQueryService{db: nil}
	_, err := svc.ListPartners(context.Background(), connect.NewRequest(&samplev1.AgentQueryServiceListPartnersRequest{}))
	if err == nil {
		t.Fatal("expected error when db is nil")
	}
	if got := connect.CodeOf(err); got != connect.CodeUnavailable {
		t.Fatalf("expected CodeUnavailable, got %v", got)
	}
}

// TestAgentMCPDescriptors verifies the compiled-in descriptor set is complete and
// dependency-ordered, and that grpcmcp accepts it and exposes exactly the read-only tool.
func TestAgentMCPDescriptors(t *testing.T) {
	fds := agentMCPDescriptors()

	idx := map[string]int{}
	for i, f := range fds.GetFile() {
		idx[f.GetName()] = i
	}
	if _, ok := idx["sample/v1/agentquery.proto"]; !ok {
		t.Fatalf("descriptor set missing agentquery.proto, got %v", idx)
	}
	if _, ok := idx["sample/v1/partner.proto"]; !ok {
		t.Fatalf("descriptor set missing the imported partner.proto, got %v", idx)
	}
	// The importer must come after its dependency so protodesc.NewFile resolves.
	if idx["sample/v1/partner.proto"] > idx["sample/v1/agentquery.proto"] {
		t.Fatal("partner.proto must precede agentquery.proto in the set")
	}

	h := newAgentMCPHandler("http://127.0.0.1:8080")
	if _, err := h.ensure(); err != nil {
		t.Fatalf("ensure with compiled-in descriptors: %v", err)
	}
}

// TestAgentMCPRoundtrip stands up the real AgentQueryService over an h2c backend (as the
// framework serves it, without reflection), points the /mcp handler at it, and drives it
// with an MCP client — proving descriptor build + tool proxying end to end.
func TestAgentMCPRoundtrip(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(samplev1connect.NewAgentQueryServiceHandler(agentQueryService{db: nil}))
	backend := httptest.NewServer(h2c.NewHandler(mux, &http2.Server{}))
	t.Cleanup(backend.Close)

	mcpHandler := newAgentMCPHandler(backend.URL)
	front := httptest.NewServer(mcpHandler)
	t.Cleanup(front.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := mcpclient.NewStreamableHttpClient(front.URL, transport.WithHTTPTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "1.0.0"}
	if _, err := client.Initialize(ctx, initReq); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	listed, err := client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	const wantTool = "sample_v1_AgentQueryService__ListPartners"
	found := false
	for _, tool := range listed.Tools {
		if tool.Name == wantTool {
			found = true
		}
	}
	if !found {
		names := make([]string, 0, len(listed.Tools))
		for _, tool := range listed.Tools {
			names = append(names, tool.Name)
		}
		t.Fatalf("expected tool %q, got %v", wantTool, names)
	}

	// db is nil, so the proxied call surfaces the "database is not configured" error as
	// tool content — proving the request reached AgentQueryService through grpcmcp.
	callReq := mcp.CallToolRequest{}
	callReq.Params.Name = wantTool
	callReq.Params.Arguments = map[string]any{}
	res, err := client.CallTool(ctx, callReq)
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	var text string
	for _, content := range res.Content {
		if tc, ok := mcp.AsTextContent(content); ok {
			text += tc.Text
		}
	}
	if !res.IsError || !strings.Contains(text, "database is not configured") {
		t.Fatalf("expected proxied db-unavailable error, got isError=%v text=%q", res.IsError, text)
	}
}
