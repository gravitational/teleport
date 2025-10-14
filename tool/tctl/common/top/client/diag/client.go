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
	"net"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	"github.com/gravitational/teleport/lib/defaults"
)

// Client is a wrapper around [*http.Client] that provides
// helpers for fetching metrics from the diagnostic endpoint of Teleport.
type Client struct {
	endpoint string
	clt      *http.Client
}

// parseAddress takes a string address and attempts to parse it into a valid URL.
// The input can either be a valid string URL or a <host>:<port> pair.
func parseAddress(addr string) (*url.URL, error) {
	u, err := url.Parse(addr)

	if err != nil || u.Scheme == "" || u.Host == "" {
		// Attempt to parse the input as a host:port tuple instead.
		_, _, err = net.SplitHostPort(addr)
		if err != nil {
			return nil, trace.Errorf("address %s is neither a valid URL nor <host>:<port>", addr)
		}

		u = &url.URL{
			Scheme: "http",
			Host:   addr,
		}
	}

	return u, nil
}

// NewClient creates a new Client for a given address.
func NewClient(addr string) (*Client, error) {
	clt, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u, err := parseAddress(addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if u.Scheme != "http" {
		return nil, trace.Errorf("unsupported scheme: %s, please provide a http address", u.Scheme)
	}

	return &Client{
		endpoint: u.JoinPath("metrics").String(),
		clt:      clt,
	}, nil
}

// GetMetrics returns prometheus metrics as a map keyed by metric name.
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

	parser := expfmt.NewTextParser(model.UTF8Validation)
	metrics, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return metrics, nil
}
