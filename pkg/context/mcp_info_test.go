package context

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPMethodInfoDecodeArguments(t *testing.T) {
	t.Run("caches decoded arguments for compatibility", func(t *testing.T) {
		info := &MCPMethodInfo{
			RawArguments: []byte(`{"owner":"github","repo":"github-mcp-server","path":"README.md"}`),
		}

		decoded, err := info.DecodeArguments()
		require.NoError(t, err)
		require.NotNil(t, decoded)
		assert.Equal(t, map[string]any{
			"owner": "github",
			"repo":  "github-mcp-server",
			"path":  "README.md",
		}, decoded)

		cached, err := info.DecodeArguments()
		require.NoError(t, err)
		assert.Equal(t, decoded, cached)
		assert.Equal(t, decoded, info.Arguments)
	})

	t.Run("returns predecoded arguments when present", func(t *testing.T) {
		arguments := map[string]any{"owner": "github"}
		info := &MCPMethodInfo{Arguments: arguments}

		decoded, err := info.DecodeArguments()
		require.NoError(t, err)
		assert.Equal(t, arguments, decoded)
	})

	t.Run("null arguments decode as nil", func(t *testing.T) {
		info := &MCPMethodInfo{RawArguments: []byte(`null`)}

		decoded, err := info.DecodeArguments()
		require.NoError(t, err)
		assert.Nil(t, decoded)
		assert.Nil(t, info.Arguments)
	})

	t.Run("non object arguments return an error", func(t *testing.T) {
		info := &MCPMethodInfo{RawArguments: []byte(`"not an object"`)}

		decoded, err := info.DecodeArguments()
		require.Error(t, err)
		assert.Nil(t, decoded)
		assert.Nil(t, info.Arguments)
	})

	t.Run("duplicate keys keep the last value", func(t *testing.T) {
		info := &MCPMethodInfo{RawArguments: []byte(`{"owner":"first","owner":"second"}`)}

		decoded, err := info.DecodeArguments()
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"owner": "second"}, decoded)
	})

	t.Run("key casing is preserved", func(t *testing.T) {
		info := &MCPMethodInfo{RawArguments: []byte(`{"owner":"lower","Owner":"upper"}`)}

		decoded, err := info.DecodeArguments()
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"owner": "lower", "Owner": "upper"}, decoded)
	})

	t.Run("concurrent decode is safe", func(t *testing.T) {
		info := &MCPMethodInfo{
			RawArguments: []byte(`{"owner":"github","repo":"github-mcp-server","nested":{"path":"README.md"}}`),
		}

		var wg sync.WaitGroup
		errors := make(chan error, 32)
		for range 32 {
			wg.Go(func() {
				decoded, err := info.DecodeArguments()
				if err != nil {
					errors <- err
					return
				}
				if decoded["owner"] != "github" {
					errors <- assert.AnError
				}
			})
		}
		wg.Wait()
		close(errors)

		for err := range errors {
			require.NoError(t, err)
		}
	})
}
