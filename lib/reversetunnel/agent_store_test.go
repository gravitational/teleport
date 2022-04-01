package reversetunnel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentStore(t *testing.T) {
	store := newAgentStore()

	agents := []*Agent{{}, {}, {}, {}, {}}

	for i, agent := range agents {
		store.add(agent)
		require.Equal(t, i+1, store.len())
	}

	_, ok := store.poplen(store.len())
	require.False(t, ok)

	agent, ok := store.poplen(store.len() - 1)
	require.True(t, ok)
	require.Equal(t, len(agents)-1, store.len())
	require.Equal(t, agents[0], agent, "first agent added is removed first.")
}
