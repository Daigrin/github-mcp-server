package context

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPMethodInfoDecodeArguments(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    map[string]any
		wantErr bool
	}{
		{name: "missing"},
		{name: "null", raw: `null`},
		{name: "empty object", raw: `{}`, want: map[string]any{}},
		{name: "owner only", raw: `{"owner":"github"}`, want: map[string]any{"owner": "github"}},
		{name: "repo only", raw: `{"repo":"server"}`, want: map[string]any{"repo": "server"}},
		{
			name: "exact and differently cased keys remain distinct",
			raw:  `{"owner":"github","Owner":"other","repo":"server","REPO":"other"}`,
			want: map[string]any{"owner": "github", "Owner": "other", "repo": "server", "REPO": "other"},
		},
		{
			name: "differently cased keys do not introduce lowercase keys",
			raw:  `{"Owner":"other","REPO":"other"}`,
			want: map[string]any{"Owner": "other", "REPO": "other"},
		},
		{
			name: "escaped keys and values",
			raw:  `{"ow\u006eer":"git\u0068ub","re\u0070o":"server","\u004fwner":null}`,
			want: map[string]any{"owner": "github", "repo": "server", "Owner": nil},
		},
		{
			name: "last duplicate wins including null",
			raw:  `{"owner":123,"owner":"github","repo":"server","repo":null}`,
			want: map[string]any{"owner": "github", "repo": nil},
		},
		{
			name: "escaped duplicate replaces exact key",
			raw:  `{"owner":"github","ow\u006eer":null,"repo":[],"repo":"server"}`,
			want: map[string]any{"owner": nil, "repo": "server"},
		},
		{
			name: "duplicate becomes object",
			raw:  `{"owner":"github","owner":{},"repo":"server"}`,
			want: map[string]any{"owner": map[string]any{}, "repo": "server"},
		},
		{
			name: "wrongly typed owner does not discard repo",
			raw:  `{"owner":123,"repo":"server"}`,
			want: map[string]any{"owner": float64(123), "repo": "server"},
		},
		{
			name: "wrongly typed repo does not discard owner",
			raw:  `{"owner":"github","repo":false}`,
			want: map[string]any{"owner": "github", "repo": false},
		},
		{
			name: "nested policy arguments are retained",
			raw:  `{"files":[{"path":".github/workflows/ci.yml"}]}`,
			want: map[string]any{"files": []any{map[string]any{"path": ".github/workflows/ci.yml"}}},
		},
		{name: "array shape", raw: `[]`, wantErr: true},
		{name: "string shape", raw: `"not an object"`, wantErr: true},
		{name: "number shape", raw: `123`, wantErr: true},
		{name: "boolean shape", raw: `true`, wantErr: true},
		{name: "malformed JSON", raw: `{"owner":"github","repo":}`, wantErr: true},
		{name: "trailing JSON", raw: `{"owner":"github"} {}`, wantErr: true},
		{name: "unrepresentable number", raw: `{"owner":"github","extra":1e1000}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := MCPMethodInfo{RawArguments: json.RawMessage(tt.raw)}
			arguments, err := info.DecodeArguments()
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, arguments, "a failed decode must not expose a partial map")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, arguments)
			}
			assert.Equal(t, MCPMethodInfo{RawArguments: json.RawMessage(tt.raw)}, info,
				"decoding must not mutate raw arguments or populate legacy fields")
		})
	}
}

func TestMCPMethodInfoDecodeArgumentsReturnsIndependentMaps(t *testing.T) {
	const raw = `{"owner":"github","files":[{"path":"README.md"}]}`
	info := MCPMethodInfo{RawArguments: json.RawMessage(raw)}

	first, err := info.DecodeArguments()
	require.NoError(t, err)
	first["owner"] = "changed"
	files, ok := first["files"].([]any)
	require.True(t, ok)
	file, ok := files[0].(map[string]any)
	require.True(t, ok)
	file["path"] = "changed"

	second, err := info.DecodeArguments()
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"owner": "github",
		"files": []any{map[string]any{"path": "README.md"}},
	}, second)
	assert.Equal(t, MCPMethodInfo{RawArguments: json.RawMessage(raw)}, info)
}

func TestMCPMethodInfoDecodeArgumentsIgnoresLegacyFields(t *testing.T) {
	for _, raw := range []string{"", `{"owner":"current"}`} {
		t.Run(raw, func(t *testing.T) {
			info := MCPMethodInfo{
				RawArguments: json.RawMessage(raw),
				Owner:        "legacy-owner",
				Repo:         "legacy-repo",
				Arguments:    map[string]any{"owner": "legacy-argument"},
			}
			arguments, err := info.DecodeArguments()
			require.NoError(t, err)
			if raw == "" {
				assert.Nil(t, arguments)
			} else {
				assert.Equal(t, map[string]any{"owner": "current"}, arguments)
			}
			assert.Equal(t, "legacy-owner", info.Owner)
			assert.Equal(t, "legacy-repo", info.Repo)
			assert.Equal(t, map[string]any{"owner": "legacy-argument"}, info.Arguments)
		})
	}
}
