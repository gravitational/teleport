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
		return "", trace.Wrap(err)
	}
	// We trim spaces because the value might end with one or many newlines
	version, err := EnsureSemver(strings.TrimSpace(string(body)))
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
