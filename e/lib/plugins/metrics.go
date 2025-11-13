package plugins

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

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

type hostedPluginsRegistry struct {
	lock       sync.Mutex
	registry   *prometheus.Registry
	collectors map[metricKey]prometheus.Collector
}

func newHostedPluginsRegistry() *hostedPluginsRegistry {
	return &hostedPluginsRegistry{
		registry:   prometheus.NewRegistry(),
		collectors: make(map[metricKey]prometheus.Collector),
	}
}

func (p *hostedPluginsRegistry) add(plugin types.Plugin, c prometheus.Collector) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	pluginLabels := prometheus.Labels{
		"pluginName": plugin.GetName(),
		"pluginType": string(plugin.GetType()),
	}

	// Although prefix and label wrapping is done by the same struct,
	// the go prometheus lib doesn't allow wrapping with prefix and labels at
	// the same time. We must chain wrappers.
	wrapped := prometheus.WrapCollectorWith(pluginLabels, c)
	p.collectors[pluginMetricKey(plugin)] = wrapped
	return p.registry.Register(wrapped)
}

func (p *hostedPluginsRegistry) remove(plugin types.Plugin) {
	p.lock.Lock()
	defer p.lock.Unlock()

	wrapped, ok := p.collectors[pluginMetricKey(plugin)]
	if !ok {
		return
	}
	p.registry.Unregister(wrapped)
	delete(p.collectors, pluginMetricKey(plugin))
}
