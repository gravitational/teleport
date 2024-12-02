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
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

// GraphClient represents generic MS API client
type GraphClient struct {
	token azcore.AccessToken
}

const (
	usersSuffix             = "users"
	groupsSuffix            = "groups"
	servicePrincipalsSuffix = "servicePrincipals"
	graphBaseURL            = "https://graph.microsoft.com/v1.0"
	httpTimeout             = time.Second * 30
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
	Context  string          `json:"@odata.context"`
	Count    int             `json:"@odata.count"`
	NextLink string          `json:"@odata.nextLink"`
	Value    json.RawMessage `json:"value"`
}

// User represents user resource
type User struct {
	ID       string       `json:"id"`
	Name     string       `json:"displayName"`
	MemberOf []Membership `json:"memberOf"`
}

type Membership struct {
	Type string `json:"@odata.type"`
	ID   string `json:"id"`
}

// request represents generic request structure
type request struct {
	// Method HTTP method
	Method string
	// URL which overrides URL construction
	URL *string
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

// GetURL builds the request URL
func (r *request) GetURL() (string, error) {
	if r.URL != nil {
		return *r.URL, nil
	}
	u, err := url.Parse(graphBaseURL)
	if err != nil {
		return "", err
	}

	data := url.Values{}
	if len(r.Expand) > 0 {
		data.Set("$expand", strings.Join(r.Expand, ","))
	}
	if r.Filter != "" {
		data.Set("$filter", r.Filter)
	}

	u.Path = u.Path + "/" + r.Path
	u.RawQuery = data.Encode()

	return u.String(), nil
}

// NewGraphClient creates MS Graph API client
func NewGraphClient(token azcore.AccessToken) *GraphClient {
	return &GraphClient{
		token: token,
	}
}

// Error returns error string
func (e graphError) Error() string {
	return e.E.Code + " " + e.E.Message
}

func (c *GraphClient) ListUsers(ctx context.Context) ([]User, error) {
	return c.listIdentities(ctx, usersSuffix, []string{"memberOf"})
}

func (c *GraphClient) ListGroups(ctx context.Context) ([]User, error) {
	return c.listIdentities(ctx, groupsSuffix, []string{"memberOf"})
}

func (c *GraphClient) ListServicePrincipals(ctx context.Context) ([]User, error) {
	return c.listIdentities(ctx, servicePrincipalsSuffix, []string{"memberOf"})
}

func (c *GraphClient) listIdentities(ctx context.Context, idType string, expand []string) ([]User, error) {
	var users []User
	var nextLink *string
	for {
		g := &genericGraphResponse{}
		req := request{
			Method:   http.MethodGet,
			Path:     idType,
			Expand:   expand,
			Response: &g,
			Err:      &graphError{},
			URL:      nextLink,
		}
		err := c.request(ctx, req)
		if err != nil {
			return nil, err
		}
		var newUsers []User
		err = json.NewDecoder(bytes.NewReader(g.Value)).Decode(&newUsers)
		if err != nil {
			return nil, err
		}
		users = append(users, newUsers...)
		if g.NextLink == "" {
			break
		}
		nextLink = &g.NextLink
	}

	return users, nil
}

// request sends the request to the graph/bot service and returns response body as bytes slice
func (c *GraphClient) request(ctx context.Context, req request) error {
	reqUrl, err := req.GetURL()
	if err != nil {
		return err
	}

	r, err := http.NewRequestWithContext(ctx, req.Method, reqUrl, strings.NewReader(req.Body))
	if err != nil {
		return err
	}

	r.Header.Set("Authorization", "Bearer "+c.token.Token)
	r.Header.Set("Content-Type", "application/json")

	client := http.Client{Timeout: httpTimeout}
	resp, err := client.Do(r)
	if err != nil {
		return err
	}

	defer func(r *http.Response) {
		_ = r.Body.Close()
	}(resp)

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
