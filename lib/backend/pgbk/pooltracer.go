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
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend/backendmetrics"
)

type connectStartKey struct{}

type poolTracer struct {
	log *slog.Logger

	connectSuccess prometheus.Observer
	connectFailure prometheus.Observer
}

var _ pgx.ConnectTracer = (*poolTracer)(nil)

func newPoolTracer(component string, log *slog.Logger) *poolTracer {
	return &poolTracer{
		log:            log,
		connectSuccess: backendmetrics.ConnectDurations.WithLabelValues(component, teleport.TagSuccess),
		connectFailure: backendmetrics.ConnectDurations.WithLabelValues(component, teleport.TagFailure),
	}
}

// TraceQueryStart implements [pgx.QueryTracer]. This is a noop and
// only exists because ConnConfig.Tracer requires a pgx.QueryTracer.
func (p *poolTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return ctx
}

// TraceQueryStart implements [pgx.QueryTracer]. This is a noop and
// only exists because ConnConfig.Tracer requires a pgx.QueryTracer.
func (p *poolTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {}

// TraceConnectStart implements [pgx.ConnectTracer]
func (p *poolTracer) TraceConnectStart(ctx context.Context, data pgx.TraceConnectStartData) context.Context {
	return context.WithValue(ctx, connectStartKey{}, time.Now())
}

// TraceConnectEnd implements [pgx.ConnectTracer]
func (p *poolTracer) TraceConnectEnd(ctx context.Context, data pgx.TraceConnectEndData) {
	start, _ := ctx.Value(connectStartKey{}).(time.Time)
	duration := time.Since(start)

	if data.Err == nil {
		p.connectSuccess.Observe(duration.Seconds())
		return
	}

	p.connectFailure.Observe(duration.Seconds())
	p.log.WarnContext(ctx, "Failed to establish Postgres connection",
		"error", data.Err,
		"duration", duration,
	)

}
