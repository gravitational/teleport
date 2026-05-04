package web

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
)

var proxyAccessGraphConnected = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Subsystem: "access_graph",
		Name:      "proxy_connected",
		Help:      "Whether the Teleport Proxy can reach the Access Graph service HTTP endpoint. Set to 1 when the proxy successfully fetches features.json, 0 otherwise.",
	},
)

func setProxyAccessGraphConnected(connected bool) {
	if connected {
		proxyAccessGraphConnected.Set(1)
	} else {
		proxyAccessGraphConnected.Set(0)
	}
}

func init() {
	// TODO(tigrato): remove this once initMinimalReverseTunnel stops calling
	// RegisterProxyWebHandlers after auth the real webh handler is created.
	prometheus.DefaultRegisterer.MustRegister(proxyAccessGraphConnected)
}
