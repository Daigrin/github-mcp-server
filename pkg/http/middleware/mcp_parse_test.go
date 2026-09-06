package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithMCPParse(t *testing.T) {
	tests := []struct {
		name            string
		method          string
		path            string
		body            string
		expectInfo      bool
		expectedMethod  string
		expectedItem    string
		expectedRaw     string
		expectedArgs    map[string]any
		expectArgsError bool
	}{
		{
			name:       "health check path is skipped",
			method:     http.MethodPost,
			path:       "/_ping",
			body:       `{"jsonrpc":"2.0","method":"tools/list"}`,
			expectInfo: false,
		},
		{
			name:       "GET request is skipped",
			method:     http.MethodGet,
			path:       "/mcp",
			body:       `{"jsonrpc":"2.0","method":"tools/list"}`,
			expectInfo: false,
		},
		{
			name:       "empty body is skipped",
			method:     http.MethodPost,
			path:       "/mcp",
			body:       "",
			expectInfo: false,
		},
		{
			name:       "invalid JSON is skipped",
			method:     http.MethodPost,
			path:       "/mcp",
			body:       "not valid json",
			expectInfo: false,
		},
		{
			name:       "malformed arguments are skipped",
			method:     http.MethodPost,
			path:       "/mcp",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"test_tool","arguments":{"owner":"github","repo":}}}`,
			expectInfo: false,
		},
		{
			name:       "trailing JSON is skipped",
			method:     http.MethodPost,
			path:       "/mcp",
			body:       `{"jsonrpc":"2.0","method":"tools/call","params":{"arguments":{"owner":"github"}}} {}`,
			expectInfo: false,
		},
		{
			name:       "non-JSON-RPC 2.0 is skipped",
			method:     http.MethodPost,
			path:       "/mcp",
			body:       `{"jsonrpc":"1.0","method":"tools/list"}`,
			expectInfo: false,
		},
		{
			name:       "empty method is skipped",
			method:     http.MethodPost,
			path:       "/mcp",
			body:       `{"jsonrpc":"2.0","method":""}`,
			expectInfo: false,
		},
		{
			name:           "tools/list parses method only",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"tools/list"}`,
			expectInfo:     true,
			expectedMethod: "tools/list",
		},
		{
			name:           "tools/call parses name",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_file_contents"}}`,
			expectInfo:     true,
			expectedMethod: "tools/call",
			expectedItem:   "get_file_contents",
		},
		{
			name:           "tools/call retains arguments for explicit decoding",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_file_contents","arguments":{"owner":"github","repo":"github-mcp-server","path":"README.md"}}}`,
			expectInfo:     true,
			expectedMethod: "tools/call",
			expectedItem:   "get_file_contents",
			expectedRaw:    `{"owner":"github","repo":"github-mcp-server","path":"README.md"}`,
			expectedArgs:   map[string]any{"owner": "github", "repo": "github-mcp-server", "path": "README.md"},
		},
		{
			name:           "tools/call retains large extra arguments fields",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_file_contents","arguments":{"owner":"github","repo":"github-mcp-server","path":"README.md","extra":"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}}}`,
			expectInfo:     true,
			expectedMethod: "tools/call",
			expectedItem:   "get_file_contents",
		},
		{
			name:            "tools/call with invalid argument shape continues with raw args",
			method:          http.MethodPost,
			path:            "/mcp",
			body:            `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_file_contents","arguments":"not an object"}}`,
			expectInfo:      true,
			expectedMethod:  "tools/call",
			expectedItem:    "get_file_contents",
			expectedRaw:     `"not an object"`,
			expectArgsError: true,
		},
		{
			name:           "tools/call retains wrongly typed owner for explicit decoding",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_file_contents","arguments":{"owner":123,"repo":"github-mcp-server"}}}`,
			expectInfo:     true,
			expectedMethod: "tools/call",
			expectedItem:   "get_file_contents",
			expectedRaw:    `{"owner":123,"repo":"github-mcp-server"}`,
			expectedArgs:   map[string]any{"owner": float64(123), "repo": "github-mcp-server"},
		},
		{
			name:           "prompts/get parses name",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"prompts/get","params":{"name":"my_prompt"}}`,
			expectInfo:     true,
			expectedMethod: "prompts/get",
			expectedItem:   "my_prompt",
		},
		{
			name:           "resources/read parses URI as item name",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"repo://github/github-mcp-server"}}`,
			expectInfo:     true,
			expectedMethod: "resources/read",
			expectedItem:   "repo://github/github-mcp-server",
		},
		{
			name:           "initialize method parses correctly",
			method:         http.MethodPost,
			path:           "/mcp",
			body:           `{"jsonrpc":"2.0","method":"initialize","params":{"capabilities":{}}}`,
			expectInfo:     true,
			expectedMethod: "initialize",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedInfo *ghcontext.MCPMethodInfo
			var infoCaptured bool

			// Create a handler that captures the MCPMethodInfo from context
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedInfo, infoCaptured = ghcontext.MCPMethod(r.Context())
				restoredBody, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				assert.Equal(t, tt.body, string(restoredBody))
				w.WriteHeader(http.StatusNoContent)
			})

			middleware := WithMCPParse()
			handler := middleware(nextHandler)

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusNoContent, rr.Code, "parsing must not block downstream handlers")
			if tt.expectInfo {
				require.True(t, infoCaptured, "MCPMethodInfo should be present in context")
				require.NotNil(t, capturedInfo)
				assert.Equal(t, tt.expectedMethod, capturedInfo.Method)
				assert.Equal(t, tt.expectedItem, capturedInfo.ItemName)
				assert.Equal(t, &ghcontext.MCPMethodInfo{
					Method:       tt.expectedMethod,
					ItemName:     tt.expectedItem,
					RawArguments: capturedInfo.RawArguments,
				}, capturedInfo, "legacy fields must remain unpopulated")
				if tt.expectedRaw != "" {
					assert.Equal(t, tt.expectedRaw, string(capturedInfo.RawArguments))
				}
				if tt.expectedArgs != nil || tt.expectArgsError {
					decodedArgs, err := capturedInfo.DecodeArguments()
					if tt.expectArgsError {
						assert.Error(t, err)
					} else {
						require.NoError(t, err)
					}
					assert.Equal(t, tt.expectedArgs, decodedArgs)
				}
			} else {
				assert.False(t, infoCaptured, "MCPMethodInfo should not be present in context")
			}
		})
	}
}

func TestWithMCPParse_ArgumentTypes(t *testing.T) {
	tests := []struct {
		name            string
		arguments       string
		owner           string
		repo            string
		expectArgsError bool
	}{
		{name: "empty object", arguments: `{}`},
		{name: "null arguments", arguments: `null`},
		{name: "array arguments", arguments: `[]`, expectArgsError: true},
		{name: "numeric arguments", arguments: `123`, expectArgsError: true},
		{name: "string arguments", arguments: `"not an object"`, expectArgsError: true},
		{name: "boolean arguments", arguments: `true`, expectArgsError: true},
		{name: "owner only", arguments: `{"owner":"github"}`, owner: "github"},
		{name: "repo only", arguments: `{"repo":"server"}`, repo: "server"},
		{name: "object owner", arguments: `{"owner":{},"repo":"server"}`, repo: "server"},
		{name: "array owner", arguments: `{"owner":[],"repo":"server"}`, repo: "server"},
		{name: "boolean owner", arguments: `{"owner":true,"repo":"server"}`, repo: "server"},
		{name: "null owner", arguments: `{"owner":null,"repo":"server"}`, repo: "server"},
		{name: "numeric repo", arguments: `{"owner":"github","repo":123}`, owner: "github"},
		{name: "object repo", arguments: `{"owner":"github","repo":{}}`, owner: "github"},
		{name: "array repo", arguments: `{"owner":"github","repo":[]}`, owner: "github"},
		{name: "boolean repo", arguments: `{"owner":"github","repo":false}`, owner: "github"},
		{name: "null repo", arguments: `{"owner":"github","repo":null}`, owner: "github"},
		{name: "both invalid", arguments: `{"owner":[],"repo":{}}`},
		{name: "last duplicate wins", arguments: `{"owner":123,"owner":"github","repo":"server","repo":null}`, owner: "github"},
		{
			name: "large nested extra arguments",
			arguments: `{"extra":[` +
				strings.Repeat(`{"owner":"ignored","repo":"ignored","content":"`+strings.Repeat("x", 1024)+`"},`, 127) +
				`{}],"owner":"github","repo":"server"}`,
			owner: "github",
			repo:  "server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"test_tool","arguments":` + tt.arguments + `}}`
			var nextCalled bool
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				nextCalled = true
				info, ok := ghcontext.MCPMethod(r.Context())
				require.True(t, ok)
				require.NotNil(t, info)
				assert.Equal(t, "tools/call", info.Method)
				assert.Equal(t, "test_tool", info.ItemName)
				assert.Equal(t, tt.arguments, string(info.RawArguments))
				assert.Equal(t, &ghcontext.MCPMethodInfo{
					Method:       "tools/call",
					ItemName:     "test_tool",
					RawArguments: info.RawArguments,
				}, info, "legacy fields must remain unpopulated")
				arguments, err := info.DecodeArguments()
				if tt.expectArgsError {
					assert.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				owner, _ := arguments["owner"].(string)
				repo, _ := arguments["repo"].(string)
				assert.Equal(t, tt.owner, owner)
				assert.Equal(t, tt.repo, repo)

				restoredBody, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				assert.Equal(t, body, string(restoredBody))
			})
			req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
			WithMCPParse()(next).ServeHTTP(httptest.NewRecorder(), req)
			assert.True(t, nextCalled)
		})
	}
}

func TestWithMCPParseRetainsLargeArgumentsWithoutMaterializingThem(t *testing.T) {
	nested := map[string]any{
		"items": []any{
			map[string]any{"payload": strings.Repeat("x", 32*1024)},
			[]any{1.0, 2.0, 3.0},
		},
	}
	rawArguments, err := json.Marshal(nested)
	require.NoError(t, err)
	tests := []struct {
		name string
		raw  string
	}{
		{name: "nested arrays and objects", raw: string(rawArguments)},
		{
			name: "large nested extra arguments",
			raw: `{"extra":[` +
				strings.Repeat(`{"owner":"ignored","repo":"ignored","content":"`+strings.Repeat("x", 1024)+`"},`, 127) +
				`{}],"owner":"github","repo":"server"}`,
		},
		{name: "unrepresentable number remains raw", raw: `{"owner":"github","extra":1e1000}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"test_tool","arguments":` + tt.raw + `}}`
			var capturedInfo *ghcontext.MCPMethodInfo
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedInfo, _ = ghcontext.MCPMethod(r.Context())
				restoredBody, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				assert.Equal(t, body, string(restoredBody))
				w.WriteHeader(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
			response := httptest.NewRecorder()
			WithMCPParse()(next).ServeHTTP(response, request)

			assert.Equal(t, http.StatusNoContent, response.Code)
			require.NotNil(t, capturedInfo)
			assert.Equal(t, &ghcontext.MCPMethodInfo{
				Method:       "tools/call",
				ItemName:     "test_tool",
				RawArguments: json.RawMessage(tt.raw),
			}, capturedInfo, "only raw metadata should be populated")
		})
	}
}

func BenchmarkParseMCPMethodInfo(b *testing.B) {
	payloads := []struct {
		name string
		args string
	}{
		{name: "small", args: `{"owner":"github","repo":"server","path":"README.md"}`},
		{
			name: "large_string",
			args: `{"owner":"github","repo":"server","content":"` + strings.Repeat("x", 128*1024) + `"}`,
		},
		{
			name: "large_nested",
			args: `{"owner":"github","repo":"server","files":[` +
				strings.Repeat(`{"path":"README.md","content":"`+strings.Repeat("x", 1024)+`"},`, 127) + `{}]}`,
		},
	}
	for _, payload := range payloads {
		b.Run(payload.name, func(b *testing.B) {
			body := []byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"test_tool","arguments":` + payload.args + `}}`)
			for _, eager := range []bool{false, true} {
				name := "parse_only"
				if eager {
					name = "eager_map_reference"
				}
				b.Run(name, func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(body)))
					for b.Loop() {
						info, err := parseMCPMethodInfo(body)
						if err != nil || info == nil {
							b.Fatalf("failed to parse MCP envelope: %v", err)
						}
						if len(info.RawArguments) != len(payload.args) {
							b.Fatal("arguments were not preserved")
						}
						// Compare envelope-only parsing with the former eager
						// map decode. Neither branch calls DecodeArguments.
						if eager {
							var args map[string]any
							if err := json.Unmarshal(info.RawArguments, &args); err != nil {
								b.Fatal(err)
							}
							if args["owner"] != "github" || args["repo"] != "server" {
								b.Fatal("unexpected repository arguments")
							}
						}
					}
				})
			}
		})
	}
}

func TestWithMCPParse_BodyRestoration(t *testing.T) {
	originalBody := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"test_tool"}}`

	var capturedBody string

	nextHandler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		capturedBody = string(body)
	})

	middleware := WithMCPParse()
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(originalBody))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, originalBody, capturedBody, "body should be restored for downstream handlers")
}

// TestWithMCPParse_WithMaxBodySize mirrors the production middleware ordering,
// where WithMaxBodySize runs ahead of WithMCPParse.
func TestWithMCPParse_WithMaxBodySize(t *testing.T) {
	const limit = 128

	buildBody := func(size int) string {
		payload := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"test_tool","arguments":{"pad":"PADDING"}}}`
		if len(payload) >= size {
			return payload
		}
		pad := strings.Repeat("x", size-len(payload))
		return strings.Replace(payload, "PADDING", "PADDING"+pad, 1)
	}

	t.Run("oversized body is rejected before parsing", func(t *testing.T) {
		body := buildBody(limit + 1)
		require.Greater(t, len(body), limit)

		var nextCalled bool
		nextHandler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			nextCalled = true
		})

		handler := WithMaxBodySize(limit)(WithMCPParse()(nextHandler))

		// An unknown length skips WithMaxBodySize's Content-Length fast path,
		// so the overflow surfaces from WithMCPParse's own read.
		req := httptest.NewRequest(http.MethodPost, "/mcp", unknownLengthBody(body))
		require.Equal(t, int64(-1), req.ContentLength, "test setup: Content-Length should be unknown")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.False(t, nextCalled, "downstream handler must not run for an oversized request")
		assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
		assert.Contains(t, rr.Body.String(), "request body too large")
	})

	t.Run("boundary-size body is parsed and preserved", func(t *testing.T) {
		body := buildBody(limit)
		require.Len(t, body, limit)

		var capturedInfo *ghcontext.MCPMethodInfo
		var capturedBody string
		nextHandler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			capturedInfo, _ = ghcontext.MCPMethod(r.Context())
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			capturedBody = string(b)
		})

		handler := WithMaxBodySize(limit)(WithMCPParse()(nextHandler))

		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		require.NotNil(t, capturedInfo, "MCPMethodInfo should be parsed for an allowed request")
		assert.Equal(t, "tools/call", capturedInfo.Method)
		assert.Equal(t, "test_tool", capturedInfo.ItemName)
		assert.Equal(t, body, capturedBody, "body should be preserved for downstream handlers")
	})
}
