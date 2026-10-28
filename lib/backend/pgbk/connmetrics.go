// Teleport
// Copyright (C) 2026  Gravitational, Inc.
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
package pgbk

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend/backendmetrics"
)

// opKind classifies operations the same way [backend.Reporter] does for the
// backend_*_seconds metrics, so the connection hold histogram can be compared
// with them.
type opKind int

const (
	opRead opKind = iota
	opBatchRead
	opWrite
	opBatchWrite
)

type operation struct {
	name string
	kind opKind
}

var (
	opGet               = operation{"Get", opRead}
	opItems             = operation{"Items", opBatchRead}
	opCreate            = operation{"Create", opWrite}
	opPut               = operation{"Put", opWrite}
	opUpdate            = operation{"Update", opWrite}
	opCompareAndSwap    = operation{"CompareAndSwap", opWrite}
	opConditionalUpdate = operation{"ConditionalUpdate", opWrite}
	opDelete            = operation{"Delete", opWrite}
	opConditionalDelete = operation{"ConditionalDelete", opWrite}
	opKeepAlive         = operation{"KeepAlive", opWrite}
	opAtomicWrite       = operation{"AtomicWrite", opWrite}
	opDeleteRange       = operation{"DeleteRange", opBatchWrite}
	opDeleteExpired     = operation{"DeleteExpired", opBatchWrite}
)

type holdObservers struct {
	success, failure prometheus.Observer
}

// connMetrics holds metric series pre-resolved at construction to remove
// need to call WithLabelValues on hot paths.
type connMetrics struct {
	acquireSuccess, acquireFailure                     prometheus.Observer
	readHold, writeHold, batchReadHold, batchWriteHold holdObservers
	inUse                                              prometheus.Gauge
}

func newConnMetrics(component string) connMetrics {
	return connMetrics{
		acquireSuccess: backendmetrics.ConnAcquireDurations.WithLabelValues(component, teleport.TagSuccess),
		acquireFailure: backendmetrics.ConnAcquireDurations.WithLabelValues(component, teleport.TagFailure),
		readHold: holdObservers{
			success: backendmetrics.ReadConnHoldDurations.WithLabelValues(component, teleport.TagSuccess),
			failure: backendmetrics.ReadConnHoldDurations.WithLabelValues(component, teleport.TagFailure),
		},
		writeHold: holdObservers{
			success: backendmetrics.WriteConnHoldDurations.WithLabelValues(component, teleport.TagSuccess),
			failure: backendmetrics.WriteConnHoldDurations.WithLabelValues(component, teleport.TagFailure),
		},
		batchReadHold: holdObservers{
			success: backendmetrics.BatchReadConnHoldDurations.WithLabelValues(component, teleport.TagSuccess),
			failure: backendmetrics.BatchReadConnHoldDurations.WithLabelValues(component, teleport.TagFailure),
		},
		batchWriteHold: holdObservers{
			success: backendmetrics.BatchWriteConnHoldDurations.WithLabelValues(component, teleport.TagSuccess),
			failure: backendmetrics.BatchWriteConnHoldDurations.WithLabelValues(component, teleport.TagFailure),
		},

		inUse: backendmetrics.ConnsAcquired.WithLabelValues(component),
	}
}

func (m *connMetrics) holdObserver(kind opKind, err error) prometheus.Observer {
	o := m.hold(kind)
	if err != nil {
		return o.failure
	}
	return o.success
}

func (m *connMetrics) hold(kind opKind) holdObservers {
	switch kind {
	case opRead:
		return m.readHold
	case opBatchRead:
		return m.batchReadHold
	case opBatchWrite:
		return m.batchWriteHold
	case opWrite:
		return m.writeHold
	}
	panic(fmt.Sprintf("unknown opKind %d", kind))
}
