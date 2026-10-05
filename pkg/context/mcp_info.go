package context

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpMethodInfoCtx string

var mcpMethodInfoCtxKey mcpMethodInfoCtx = "mcpmethodinfo"

// MCPMethodInfo contains pre-parsed MCP method information extracted from the JSON-RPC request.
// This is populated early in the request lifecycle to enable:
//   - Inventory filtering via ForMCPRequest (only register needed tools/resources/prompts)
//   - Avoiding duplicate JSON envelope parsing in downstream middleware
//   - Performance optimization for per-request server creation
type MCPMethodInfo struct {
	// Method is the MCP method being called (e.g., "tools/call", "tools/list", "initialize")
	Method string
	// ItemName is the name of the specific item being accessed (tool name, resource URI, prompt name)
	// Only populated for call/get methods (tools/call, prompts/get, resources/read)
	ItemName string
	// Owner and Repo are best-effort exact-key string values from tools/call arguments.
	// Invalid or missing values are represented by an empty string.
	Owner string
	Repo  string
	// Arguments contains the decoded tools/call arguments after DecodeArguments is
	// called. The returned map is shared and must not be mutated.
	//
	// Prefer RawArguments and DecodeArguments in new code.
	Arguments map[string]any
	// RawArguments contains the unmaterialized tool arguments for tools/call requests.
	RawArguments json.RawMessage

	decodeMu         sync.Mutex
	decodeDone       bool
	decodeErr        error
	decodedArguments map[string]any
	// ProtocolVersion and ClientCapabilities describe the requesting MCP client
	// when stateless HTTP parsing makes them available before registration.
	ProtocolVersion    string
	ClientCapabilities *mcp.ClientCapabilities
}

// DecodeArguments materializes tool arguments when request middleware needs
// call-specific values. Concurrent calls on the same MCPMethodInfo are safe.
// The returned map is cached and shared; callers must treat it as read-only and
// must not mutate MCPMethodInfo argument fields after the first call.
// Invalid argument shapes are returned to the caller so the request can
// continue to the tool handler's normal validation path.
func (info *MCPMethodInfo) DecodeArguments() (map[string]any, error) {
	info.decodeMu.Lock()
	defer info.decodeMu.Unlock()

	if info.decodeDone {
		return info.decodedArguments, info.decodeErr
	}
	if info.Arguments != nil {
		info.decodedArguments = info.Arguments
		info.decodeDone = true
		return info.decodedArguments, nil
	}
	if len(info.RawArguments) == 0 {
		info.decodeDone = true
		return nil, nil
	}

	var arguments map[string]any
	if err := json.Unmarshal(info.RawArguments, &arguments); err != nil {
		info.decodeErr = err
		info.decodeDone = true
		return nil, err
	}
	info.decodedArguments = arguments
	info.Arguments = arguments
	info.decodeDone = true

	return info.decodedArguments, nil
}

// WithMCPMethodInfo stores the MCPMethodInfo in the context.
func WithMCPMethodInfo(ctx context.Context, info *MCPMethodInfo) context.Context {
	return context.WithValue(ctx, mcpMethodInfoCtxKey, info)
}

// MCPMethod retrieves the MCPMethodInfo from the context.
func MCPMethod(ctx context.Context) (*MCPMethodInfo, bool) {
	if info, ok := ctx.Value(mcpMethodInfoCtxKey).(*MCPMethodInfo); ok {
		return info, true
	}
	return nil, false
}
