/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package basichttp

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
)

// Client extends the regular http.Client by adding a GetContent method that does
// a GET query to a given URL and returns an error if the status is non-200.
// This is typically used to retrieve small files stored in a S3 bucket like the
// maintenance.BasicHTTPMaintenanceTrigger or the version.BasicHTTPVersionGetter
// are doing.
type Client struct {
	*http.Client
}

// GetContent sends a GET HTTP request and fails if the response is not 200.
func (c *Client) GetContent(ctx context.Context, targetURL url.URL) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL.String(), nil)
	if err != nil {
		return []byte{}, trace.Wrap(err)
	}
	res, err := c.Do(req)
	if err != nil {
		return []byte{}, trace.Wrap(err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return []byte{}, trace.Wrap(err)
	}

	if res.StatusCode != http.StatusOK {
		return []byte{}, trace.Errorf("non-200 status code received: '%d'", res.StatusCode)
	}

	return body, nil
}
