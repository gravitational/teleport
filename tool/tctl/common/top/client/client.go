// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package client

import (
	"context"
	"net/url"

	dto "github.com/prometheus/client_model/go"

	"github.com/gravitational/teleport/lib/client/debug"
	"github.com/gravitational/teleport/tool/tctl/common/top/client/http"
	"github.com/gravitational/trace"
)

type MetricCient interface {
	GetMetrics(context.Context) (map[string]*dto.MetricFamily, error)
}

// Create metrics client based on address
func NewMetricCient(addr string) (MetricCient, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch u.Scheme {
	case "unix":
		// For unix, expect: "unix:///var/lib/d.sock"
		return debug.NewClient(u.Path), nil
	default:
		// For anything else try the http client
		return http.NewClient(addr)
	}
}
