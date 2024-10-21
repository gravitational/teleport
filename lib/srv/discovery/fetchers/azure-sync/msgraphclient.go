/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package azure_sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GraphClient represents generic MS API client
type GraphClient struct {
	token   azcore.AccessToken
	baseURL string
}

const (
	graphBaseURL = "https://graph.microsoft.com/v1.0"
	httpTimeout  = time.Second * 30
)

// graphError represents MS Graph error
type graphError struct {
	E struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// genericGraphResponse represents the utility struct for parsing MS Graph API response
type genericGraphResponse struct {
	Context string          `json:"@odata.context"`
	Count   int             `json:"@odata.count"`
	Value   json.RawMessage `json:"value"`
}

// User represents user resource
type User struct {
	ID   string `json:"id"`
	Name string `json:"displayName"`
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
	Response interface{}
	// Err represents template structure for an error
	Err error
	// SuccessCode http code representing success
	SuccessCode int
}

// NewGraphClient creates MS Graph API client
func NewGraphClient(token azcore.AccessToken) *GraphClient {
	return &GraphClient{
		token:   token,
		baseURL: graphBaseURL,
	}
}

// Error returns error string
func (e graphError) Error() string {
	return e.E.Code + " " + e.E.Message
}

func (c *GraphClient) ListUsers(ctx context.Context) ([]User, error) {
	g := &genericGraphResponse{}
	request := request{
		Method:   http.MethodGet,
		Path:     "users",
		Response: &g,
		Err:      &graphError{},
	}
	err := c.request(ctx, request)
	if err != nil {
		return nil, err
	}

	var users []User
	err = json.NewDecoder(bytes.NewReader(g.Value)).Decode(&users)
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, err
	}

	if len(users) > 1 {
		return nil, err
	}

	return users, nil
}

// buildURL builds the request URL
func (c *GraphClient) buildURL(request request) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
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
func (c *GraphClient) request(ctx context.Context, req request) error {
	client := http.Client{Timeout: httpTimeout}

	url, err := c.buildURL(req)
	if err != nil {
		return err
	}

	r, err := http.NewRequestWithContext(ctx, req.Method, url, strings.NewReader(req.Body))
	if err != nil {
		return err
	}

	r.Header.Set("Authorization", "Bearer "+c.token.Token)
	r.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(r)
	if err != nil {
		return err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	expectedCode := req.SuccessCode
	if expectedCode == 0 {
		expectedCode = http.StatusOK
	}

	if expectedCode == resp.StatusCode {
		if req.Response == nil {
			return nil
		}

		err := json.NewDecoder(bytes.NewReader(b)).Decode(req.Response)
		if err != nil {
			return err
		}
	} else {
		if req.Err == nil {
			return fmt.Errorf("Error requesting MS Graph API: %v", string(b))
		}

		err := json.NewDecoder(bytes.NewReader(b)).Decode(req.Err)
		if err != nil {
			return err
		}

		if req.Err.Error() == "" {
			return fmt.Errorf("Error requesting MS Graph API. Expected response code was %v, but is %v", expectedCode, resp.StatusCode)
		}

		return req.Err
	}

	return nil
}
