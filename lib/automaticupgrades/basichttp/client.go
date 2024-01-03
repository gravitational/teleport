/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
