package plugins

import (
	"testing"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func testRegistry(nonce string) *prometheus.Registry {
	registry := prometheus.NewRegistry()
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_counter_" + nonce,
		Help: "test_counter",
	})
	counter.Add(1)
	registry.MustRegister(counter)
	return registry
}

func TestPluginMetricGatherer(t *testing.T) {
	t.Parallel()

	// Test setup: create fixtures
	pluginA := &types.PluginV1{
		Metadata: types.Metadata{
			Name: "my-plugin-A",
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_SlackAccessPlugin{},
		},
	}
	pluginB := &types.PluginV1{
		Metadata: types.Metadata{
			Name: "my-plugin-B",
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Jira{},
		},
	}

	t.Run("no plugin registered", func(t *testing.T) {
		pluginRegistry := newHostedPluginsRegistry()

		metrics, err := pluginRegistry.registry.Gather()
		require.NoError(t, err)
		require.Empty(t, metrics)
	})

	t.Run("several plugins registered", func(t *testing.T) {
		pluginRegistry := newHostedPluginsRegistry()

		pluginANonce := uuid.NewString()
		require.NoError(t, pluginRegistry.add(pluginA, testRegistry(pluginANonce)))
		pluginBNonce := uuid.NewString()
		require.NoError(t, pluginRegistry.add(pluginB, testRegistry(pluginBNonce)))

		metrics, err := pluginRegistry.registry.Gather()
		require.NoError(t, err)
		require.Len(t, metrics, 2)
	})

	t.Run("plugin registered twice", func(t *testing.T) {
		pluginRegistry := newHostedPluginsRegistry()

		pluginANonce := uuid.NewString()
		require.NoError(t, pluginRegistry.add(pluginA, testRegistry(pluginANonce)))
		require.Error(t, pluginRegistry.add(pluginA, testRegistry(pluginANonce)))

		metrics, err := pluginRegistry.registry.Gather()
		require.NoError(t, err)
		require.Len(t, metrics, 1)
	})

	t.Run("plugin unregistered", func(t *testing.T) {
		pluginRegistry := newHostedPluginsRegistry()

		pluginANonce := uuid.NewString()
		require.NoError(t, pluginRegistry.add(pluginA, testRegistry(pluginANonce)))
		pluginRegistry.remove(pluginA)

		metrics, err := pluginRegistry.registry.Gather()
		require.NoError(t, err)
		require.Empty(t, metrics)
	})

	t.Run("plugin unregistered twice", func(t *testing.T) {
		pluginRegistry := newHostedPluginsRegistry()

		pluginANonce := uuid.NewString()
		require.NoError(t, pluginRegistry.add(pluginA, testRegistry(pluginANonce)))
		pluginRegistry.remove(pluginA)
		pluginRegistry.remove(pluginA)

		metrics, err := pluginRegistry.registry.Gather()
		require.NoError(t, err)
		require.Empty(t, metrics)
	})
}
