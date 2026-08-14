package plugins

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestPluginMetricGatherer(t *testing.T) {
	t.Parallel()

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

		metrics, err := pluginRegistry.Gather()
		require.NoError(t, err)
		require.Empty(t, metrics)
	})

	t.Run("several plugins registered", func(t *testing.T) {
		pluginRegistry := newHostedPluginsRegistry()

		regA := pluginRegistry.add(pluginA)
		regA.Register(prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_counter_a", Help: "test_counter",
		}))
		regB := pluginRegistry.add(pluginB)
		regB.Register(prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_counter_b", Help: "test_counter",
		}))

		metrics, err := pluginRegistry.Gather()
		require.NoError(t, err)
		require.Len(t, metrics, 2)
	})

	t.Run("plugin registered twice", func(t *testing.T) {
		pluginRegistry := newHostedPluginsRegistry()

		reg1 := pluginRegistry.add(pluginA)
		reg1.Register(prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_counter", Help: "test_counter",
		}))

		reg2 := pluginRegistry.add(pluginA)
		reg2.Register(prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_counter", Help: "test_counter",
		}))

		metrics, err := pluginRegistry.Gather()
		require.NoError(t, err)
		require.Len(t, metrics, 1)
	})

	t.Run("plugin unregistered", func(t *testing.T) {
		pluginRegistry := newHostedPluginsRegistry()

		reg := pluginRegistry.add(pluginA)
		reg.Register(prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_counter", Help: "test_counter",
		}))

		pluginRegistry.remove(pluginA)

		metrics, err := pluginRegistry.Gather()
		require.NoError(t, err)
		require.Empty(t, metrics)
	})

	t.Run("plugin unregistered twice", func(t *testing.T) {
		pluginRegistry := newHostedPluginsRegistry()

		reg := pluginRegistry.add(pluginA)
		reg.Register(prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_counter", Help: "test_counter",
		}))

		pluginRegistry.remove(pluginA)
		pluginRegistry.remove(pluginA)

		metrics, err := pluginRegistry.Gather()
		require.NoError(t, err)
		require.Empty(t, metrics)
	})

	// Reproduces the production scenario: per-plugin registry starts empty,
	// gets metrics later, then is replaced on plugin restart.
	t.Run("empty registry replaced on plugin restart", func(t *testing.T) {
		pluginRegistry := newHostedPluginsRegistry()

		// First start: registry is empty initially, metrics added later
		reg1 := pluginRegistry.add(pluginA)
		reg1.Register(prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_metric", Help: "test metric help",
		}))

		metrics, err := pluginRegistry.Gather()
		require.NoError(t, err)
		require.Len(t, metrics, 1)

		// Plugin restarts: new registry replaces old one
		reg2 := pluginRegistry.add(pluginA)
		reg2.Register(prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_metric", Help: "test metric help",
		}))

		// Should succeed without duplicate errors
		metrics, err = pluginRegistry.Gather()
		require.NoError(t, err)
		require.Len(t, metrics, 1)
	})

	t.Run("empty registry removed", func(t *testing.T) {
		pluginRegistry := newHostedPluginsRegistry()

		reg := pluginRegistry.add(pluginA)
		reg.Register(prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_metric", Help: "test metric help",
		}))

		metrics, err := pluginRegistry.Gather()
		require.NoError(t, err)
		require.Len(t, metrics, 1)

		pluginRegistry.remove(pluginA)

		metrics, err = pluginRegistry.Gather()
		require.NoError(t, err)
		require.Empty(t, metrics)
	})
}
