package context

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPMethodInfoDecodeArgumentsCachesResult(t *testing.T) {
	info := &MCPMethodInfo{RawArguments: []byte(`{"owner":"github","repo":"github-mcp-server"}`)}

	decoded, err := info.DecodeArguments()
	require.NoError(t, err)
	require.NotNil(t, decoded)
	assert.Equal(t, "github", decoded["owner"])
	assert.Same(t, decoded, info.Arguments)

	cached, err := info.DecodeArguments()
	require.NoError(t, err)
	assert.Same(t, decoded, cached)
}

func TestMCPMethodInfoDecodeArgumentsUsesExistingArguments(t *testing.T) {
	arguments := map[string]any{"owner": "github"}
	info := &MCPMethodInfo{Arguments: arguments}

	decoded, err := info.DecodeArguments()
	require.NoError(t, err)
	assert.Same(t, arguments, decoded)
}

func TestMCPMethodInfoDecodeArgumentsCachesErrors(t *testing.T) {
	info := &MCPMethodInfo{RawArguments: []byte(`["not","an","object"]`)}

	_, err := info.DecodeArguments()
	require.Error(t, err)
	_, cachedErr := info.DecodeArguments()
	assert.Same(t, err, cachedErr)
}

func TestMCPMethodInfoDecodeArgumentsConcurrentCalls(t *testing.T) {
	info := &MCPMethodInfo{
		RawArguments: []byte(`{"owner":"github","repo":"github-mcp-server","nested":{"path":"README.md"}}`),
	}

	var wg sync.WaitGroup
	errors := make(chan error, 32)
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			decoded, err := info.DecodeArguments()
			if err != nil {
				errors <- err
				return
			}
			if decoded["owner"] != "github" {
				errors <- assert.AnError
			}
		}()
	}
	wg.Wait()
	close(errors)

	for err := range errors {
		require.NoError(t, err)
	}
}
