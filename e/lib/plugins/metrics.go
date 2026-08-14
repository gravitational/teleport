package plugins

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/gravitational/teleport/api/types"
)

type metricKey struct {
	pluginName string
	pluginType types.PluginType
}

func pluginMetricKey(p types.Plugin) metricKey {
	return metricKey{
		pluginName: p.GetName(),
		pluginType: p.GetType(),
	}
}

// hostedPluginsRegistry manages per-plugin metric registries and implements
// [prometheus.Gatherer] by delegating to all registered per-plugin registries.
type hostedPluginsRegistry struct {
	lock       sync.Mutex
	registries map[metricKey]*prometheus.Registry
}

func newHostedPluginsRegistry() *hostedPluginsRegistry {
	return &hostedPluginsRegistry{
		registries: make(map[metricKey]*prometheus.Registry),
	}
}

// add registers a per-plugin prometheus registry. If a registry already
// exists for the same plugin, it is replaced.
// The returned registerer wraps the registry with pluginName/pluginType
// labels so that all metrics registered through it carry those labels.
func (p *hostedPluginsRegistry) add(plugin types.Plugin) prometheus.Registerer {
	p.lock.Lock()
	defer p.lock.Unlock()

	key := pluginMetricKey(plugin)
	registry := prometheus.NewRegistry()
	p.registries[key] = registry

	return prometheus.WrapRegistererWith(prometheus.Labels{
		"pluginName": plugin.GetName(),
		"pluginType": string(plugin.GetType()),
	}, registry)
}

func (p *hostedPluginsRegistry) remove(plugin types.Plugin) {
	p.lock.Lock()
	defer p.lock.Unlock()
	delete(p.registries, pluginMetricKey(plugin))
}

// Gather implements [prometheus.Gatherer].
func (p *hostedPluginsRegistry) Gather() ([]*dto.MetricFamily, error) {
	p.lock.Lock()
	gatherers := make(prometheus.Gatherers, 0, len(p.registries))
	for _, c := range p.registries {
		gatherers = append(gatherers, c)
	}
	p.lock.Unlock()

	return gatherers.Gather()
}
