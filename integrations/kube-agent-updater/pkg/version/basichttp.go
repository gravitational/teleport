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

	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/basichttp"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/cache"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/constants"
)

// basicHTTPVersionClient retrieves the version from an HTTP endpoint
// it should not be invoked immediately and must be wrapped in a cache layer in
// order to avoid spamming the version server in case of reconciliation errors.
// use BasicHTTPVersionGetter if you need to get a version.
type basicHTTPVersionClient struct {
	baseURL      *url.URL
	client       *basichttp.Client
	extraHeaders map[string]string
}

// Get sends an HTTP GET request and returns the version prefixed by "v".
// It expects the endpoint to be unauthenticated, return 200 and the response
// body to contain a valid semver tag without the "v".
func (b *basicHTTPVersionClient) Get(ctx context.Context) (string, error) {
	versionURL := b.baseURL.JoinPath(constants.VersionPath)
	body, err := b.client.GetContentWithHeaders(ctx, *versionURL, b.extraHeaders)
	if err != nil {
		return "", trace.Wrap(err)
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
	client        *basicHTTPVersionClient
	versionGetter func(context.Context) (string, error)
}

func (g *BasicHTTPVersionGetter) GetVersion(ctx context.Context) (string, error) {
	return g.versionGetter(ctx)
}

func (g *BasicHTTPVersionGetter) SetHeader(header, value string) {
	if g.client.extraHeaders[header] == value {
		return
	}

	// Reinitialize the cache with the new getter function
	g.client.extraHeaders[header] = value
	g.versionGetter = cache.NewTimedMemoize[string](g.client.Get, constants.CacheDuration).Get
}

func NewBasicHTTPVersionGetter(baseURL *url.URL) *BasicHTTPVersionGetter {
	client := &http.Client{
		Timeout: constants.HTTPTimeout,
	}
	httpVersionClient := &basicHTTPVersionClient{
		baseURL:      baseURL,
		client:       &basichttp.Client{Client: client},
		extraHeaders: make(map[string]string),
	}

	return &BasicHTTPVersionGetter{
		client:        httpVersionClient,
		versionGetter: cache.NewTimedMemoize[string](httpVersionClient.Get, constants.CacheDuration).Get,
	}
}
