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

package msgraphapi

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

// Client represents generic MS API client
type Client struct {
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

// NewClient creates MS Graph API client
func NewClient(token azcore.AccessToken) *Client {
	return &Client{
		token:   token,
		baseURL: graphBaseURL,
	}
}

// Error returns error string
func (e graphError) Error() string {
	return e.E.Code + " " + e.E.Message
}

// GetUserByEmail searches a user by email
func (c *Client) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	g := &genericGraphResponse{}

	request := request{
		Method:   http.MethodGet,
		Path:     "users",
		Filter:   "mail eq '" + email + "'",
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
		return nil, fmt.Errorf("User by email %v not found", email)
	}

	if len(users) > 1 {
		return nil, fmt.Errorf("There is more than one user with email eq %v", email)
	}

	return &users[0], nil
}

// GetUserByID returns a user by ID
func (c *Client) GetUserByID(ctx context.Context, id string) (*User, error) {
	g := &genericGraphResponse{}

	request := request{
		Method:   http.MethodGet,
		Path:     "users",
		Filter:   "id eq '" + id + "'",
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

	return &users[0], nil
}

// buildURL builds the request URL
func (c *Client) buildURL(request request) (string, error) {
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
func (c *Client) request(ctx context.Context, req request) error {
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
