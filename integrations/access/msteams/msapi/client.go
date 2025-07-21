// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package msapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/integrations/lib/backoff"
)

// Client represents generic MS API client
type Client struct {
	token   tokenWithTTL
	baseURL string
	config  Config
}

// request represents generic request structure
type request struct {
	// Method HTTP method
	Method string
	// Path to a resource
	Path string
	// Expand $expand value
	Expand []string
	// Filter $filter value
	Filter string
	// Body request body
	Body string
	// Response represents template structure for a response
	Response any
	// Err represents template structure for an error
	Err error
	// SuccessCode http code representing success
	SuccessCode int
}

// buildURL builds the request URL
func (c *Client) buildURL(request request) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	data := url.Values{}
	if len(request.Expand) > 0 {
		data.Set("$expand", strings.Join(request.Expand, ","))
	}
	if request.Filter != "" {
		data.Set("$filter", request.Filter)
	}

	u.Path = u.Path + "/" + request.Path
	u.RawQuery = data.Encode()

	return u.String(), nil
}

// request sends the request to the graph/bot service and returns response body as bytes slice
func (c *Client) request(ctx context.Context, request request) error {
	client := http.Client{Timeout: httpTimeout}

	url, err := c.buildURL(request)
	if err != nil {
		return trace.Wrap(err)
	}

	token, err := c.token.Bearer(ctx, c.config)
	if err != nil {
		return trace.Wrap(err)
	}

	r, err := http.NewRequestWithContext(ctx, request.Method, url, strings.NewReader(request.Body))
	if err != nil {
		return trace.Wrap(err)
	}

	r.Header.Set("Authorization", token)
	r.Header.Set("Content-Type", "application/json")

	backoff := backoff.NewDecorr(backoffBase, backoffMax, clockwork.NewRealClock())

	for {
		resp, err := client.Do(r)
		if err != nil {
			return trace.Wrap(err)
		}

		defer resp.Body.Close()

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return trace.Wrap(err)
		}

		if resp.StatusCode > 499 || resp.StatusCode == http.StatusTooManyRequests {
			err := backoff.Do(ctx)
			if err != nil {
				return trace.Wrap(err)
			}
			continue
		}

		expectedCode := request.SuccessCode
		if expectedCode == 0 {
			expectedCode = http.StatusOK
		}

		if expectedCode == resp.StatusCode {
			if request.Response == nil {
				return nil
			}

			err := json.NewDecoder(bytes.NewReader(b)).Decode(request.Response)
			if err != nil {
				return trace.Wrap(err)
			}
		} else {
			if request.Err == nil {
				return trace.Errorf("Error requesting MS Graph API: %v", string(b))
			}

			err := json.NewDecoder(bytes.NewReader(b)).Decode(request.Err)
			if err != nil {
				return trace.Wrap(err)
			}

			if request.Err.Error() == "" {
				return trace.Errorf("Error requesting MS Graph API. Expected response code was %v, but is %v", expectedCode, resp.StatusCode)
			}

			return request.Err
		}

		return nil
	}
}
