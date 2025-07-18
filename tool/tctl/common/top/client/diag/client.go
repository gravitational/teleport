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

package diag

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	"github.com/gravitational/teleport/lib/defaults"
)

type Client struct {
	endpoint string
	clt      *http.Client
}

func NewClient(addr string) (*Client, error) {
	clt, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u, err := url.Parse(addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u.Path = "metrics"

	return &Client{
		endpoint: u.String(),
		clt:      clt,
	}, nil
}

func (c *Client) GetMetrics(ctx context.Context) (map[string]*dto.MetricFamily, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, http.NoBody)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.clt.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	var parser expfmt.TextParser
	metrics, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return metrics, nil
}
