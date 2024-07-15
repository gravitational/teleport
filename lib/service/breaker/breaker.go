// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package breaker

import (
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/types"
)

var connectorExecutions = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: teleport.MetricNamespace,
	Subsystem: "breaker",
	Name:      "connector_executions_total",
	Help:      "Client requests per system role, state of the breaker and success as interpreted by the breaker.",
}, []string{"role", "state", "success"})

var registerOnce sync.Once

func ensureRegistered() {
	registerOnce.Do(func() {
		prometheus.MustRegister(connectorExecutions)
	})
}

// InstrumentBreakerForConnector returns a copy of a [breaker.Config] that
// counts client "executions" (i.e. requests or streams) that go through the
// breaker, attributing the count to the given system role.
func InstrumentBreakerForConnector(role types.SystemRole, cfg breaker.Config) breaker.Config {
	ensureRegistered()

	cfg = cfg.Clone()
	cfg.OnExecute = func(success bool, state breaker.State) {
		connectorExecutions.WithLabelValues(role.String(), state.String(), strconv.FormatBool(success)).Inc()
	}
	return cfg
}
