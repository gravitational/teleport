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

package automaticupgrades

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// stableCloudVersionBaseURL is the base URL for the server that returns the current stable/cloud version.
	stableCloudVersionBaseURL = "https://updates.releases.teleport.dev"

	// stableCloudVersionURL is the URL that returns the current stable/cloud version.
	stableCloudVersionURL = "/v1/stable/cloud/version"
)

// Version returns the version that should be used for installing Teleport Services
// This is used when installing agents using scripts.
// Even when Teleport Auth/Proxy is using vX, the agents must always respect this version.
func Version(ctx context.Context, baseURL string) (string, error) {
	if baseURL == "" {
		baseURL = stableCloudVersionBaseURL
	}

	fullURL, err := url.JoinPath(baseURL, stableCloudVersionURL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer resp.Body.Close()

	body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("invalid status code %d, body: %s", resp.StatusCode, string(body))
	}

	versionString := strings.TrimSpace(string(body))

	return versionString, trace.Wrap(err)
}
