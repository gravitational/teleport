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

package version

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/automaticupgrades/basichttp"
	"github.com/gravitational/teleport/lib/automaticupgrades/cache"
	"github.com/gravitational/teleport/lib/automaticupgrades/constants"
)

// basicHTTPVersionClient retrieves the version from an HTTP endpoint
// it should not be invoked immediately and must be wrapped in a cache layer in
// order to avoid spamming the version server in case of reconciliation errors.
// use BasicHTTPVersionGetter if you need to get a version.
type basicHTTPVersionClient struct {
	baseURL *url.URL
	client  *basichttp.Client
}

// Get sends an HTTP GET request and returns the version prefixed by "v".
// It expects the endpoint to be unauthenticated, return 200 and the response
// body to contain a valid semver tag without the "v".
func (b *basicHTTPVersionClient) Get(ctx context.Context) (string, error) {
	versionURL := b.baseURL.JoinPath(constants.VersionPath)
	body, err := b.client.GetContent(ctx, *versionURL)
	if err != nil {
		return "", trace.Wrap(err, "failed to get version from %s", versionURL)
	}
	response := string(body)
	if response == constants.NoVersion {
		return "", &NoNewVersionError{Message: "version server did not advertise a version"}
	}
	// We trim spaces because the value might end with one or many newlines
	version, err := EnsureSemver(strings.TrimSpace(response))
	return version, trace.Wrap(err)
}

// BasicHTTPVersionGetter gets the version from an HTTP response containing
// only the version. This is used typically to get version from a file hosted
// in a s3 bucket or raw file served over HTTP.
// BasicHTTPVersionGetter uses basicHTTPVersionClient and wraps it in a cache
// in order to mitigate the impact of too frequent reconciliations.
// The structure implements the version.Getter interface.
type BasicHTTPVersionGetter struct {
	versionGetter func(context.Context) (string, error)
}

func (g BasicHTTPVersionGetter) GetVersion(ctx context.Context) (string, error) {
	return g.versionGetter(ctx)
}

func NewBasicHTTPVersionGetter(baseURL *url.URL) Getter {
	client := &http.Client{
		Timeout: constants.HTTPTimeout,
	}
	httpVersionClient := &basicHTTPVersionClient{
		baseURL: baseURL,
		client:  &basichttp.Client{Client: client},
	}

	return BasicHTTPVersionGetter{cache.NewTimedMemoize[string](httpVersionClient.Get, constants.CacheDuration).Get}
}
