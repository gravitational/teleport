package reversetunnel

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentStorePopLen(t *testing.T) {
	store := newAgentStore()

	agents := []*agent{{}, {}, {}, {}, {}}

	for i := range agents {
		store.add(agents[i])
		require.Equal(t, i+1, store.len())
	}

	_, ok := store.poplen(store.len())
	require.False(t, ok)

	agent, ok := store.poplen(store.len() - 1)
	require.True(t, ok)
	require.Equal(t, len(agents)-1, store.len())
	require.Equal(t, agents[0], agent, "first agent added is removed first.")
}

func TestAgentStoreRace(t *testing.T) {
	store := newAgentStore()
	agents := []*agent{{}, {}, {}, {}, {}}

	wg := &sync.WaitGroup{}
	for i := range agents {
		wg.Add(1)
		go func(i int) {
			store.add(agents[i])
			wg.Done()
		}(i)
	}

	wg.Wait()

	wg = &sync.WaitGroup{}
	for i := range agents {
		wg.Add(1)
		go func(i int) {
			ok := store.remove(agents[i])
			require.True(t, ok)
			wg.Done()
		}(i)
	}

	wg.Wait()
}
