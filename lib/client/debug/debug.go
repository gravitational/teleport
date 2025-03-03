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

package debug

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
)

// SupportedProfiles list of supported pprof profiles that can be collected.
// This list is composed by runtime/pprof.Profile and http/pprof definitions.
var SupportedProfiles = map[string]struct{}{
	"allocs":       {},
	"block":        {},
	"cmdline":      {},
	"goroutine":    {},
	"heap":         {},
	"mutex":        {},
	"profile":      {},
	"threadcreate": {},
	"trace":        {},
}

// Client represents the debug service client.
type Client struct {
	clt *http.Client
}

// NewClient generates a new debug service client.
func NewClient(socketPath string) *Client {
	return &Client{
		clt: &http.Client{
			Timeout: apidefaults.DefaultIOTimeout,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", socketPath)
				},
				DisableKeepAlives: true,
			},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return trace.Errorf("redirect via socket not allowed")
			},
		},
	}
}

// SetLogLevel changes the application's log level and a change status message.
func (c *Client) SetLogLevel(ctx context.Context, level string) (string, error) {
	resp, err := c.do(ctx, http.MethodPut, url.URL{Path: "/log-level"}, []byte(level))
	if err != nil {
		return "", trace.Wrap(err)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	defer resp.Body.Close()
	if err != nil {
		return "", trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("Unable to change log level: %s", respBody)
	}

	return string(respBody), nil
}

// GetLogLevel fetches the current log level.
func (c *Client) GetLogLevel(ctx context.Context) (string, error) {
	resp, err := c.do(ctx, http.MethodGet, url.URL{Path: "/log-level"}, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	defer resp.Body.Close()
	if err != nil {
		return "", trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("Unable to fetch log level: %s", respBody)
	}

	return string(respBody), nil
}

// CollectProfile collects a pprof profile.
func (c *Client) CollectProfile(ctx context.Context, profileName string, seconds int) ([]byte, error) {
	u := url.URL{
		Path: "/debug/pprof/" + profileName,
	}

	if _, ok := SupportedProfiles[profileName]; !ok {
		return nil, trace.BadParameter("%q profile not supported", profileName)
	}

	if seconds > 0 {
		qs := url.Values{}
		qs.Add("seconds", strconv.Itoa(seconds))
		u.RawQuery = qs.Encode()
	}

	resp, err := c.do(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("Unable to collect profile %q: %s", profileName, result)
	}

	return result, nil
}

// Readiness describes the readiness of the Teleport instance.
type Readiness struct {
	// Ready is true if the instance is ready.
	// This field is only set by clients, based on status.
	Ready bool `json:"-"`
	// Status provides more detail about the readiness status.
	Status string `json:"status"`
	// PID is the process PID
	PID int `json:"pid"`
}

// GetReadiness returns true if the Teleport service is ready.
func (c *Client) GetReadiness(ctx context.Context) (Readiness, error) {
	var ready Readiness
	resp, err := c.do(ctx, http.MethodGet, url.URL{Path: "/readyz"}, nil)
	if err != nil {
		return ready, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ready, trace.NotFound("readiness endpoint not found")
	}
	ready.Ready = resp.StatusCode == http.StatusOK
	err = json.NewDecoder(resp.Body).Decode(&ready)
	if err != nil {
		return ready, trace.Wrap(err)
	}
	return ready, nil
}

func (c *Client) do(ctx context.Context, method string, u url.URL, body []byte) (*http.Response, error) {
	u.Scheme = "http"
	u.Host = "debug"

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewBuffer(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.clt.Do(req)
	if err != nil {
		return nil, trace.Wrap(trace.ConvertSystemError(err))
	}

	return resp, nil
}
