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

package http

import (
	"context"
	"net/url"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
)

type Client struct {
	clt *roundtrip.Client
}

func NewClient(addr string) (*Client, error) {
	clt, err := roundtrip.NewClient(addr, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Client{
		clt: clt,
	}, nil
}

func (c *Client) GetMetrics(ctx context.Context) (map[string]*dto.MetricFamily, error) {

	re, err := c.clt.Get(ctx, c.clt.Endpoint("metrics"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err))
	}

	var parser expfmt.TextParser
	metrics, err := parser.TextToMetricFamilies(re.Reader())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return metrics, nil
}
