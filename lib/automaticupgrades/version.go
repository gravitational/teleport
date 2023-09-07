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

	// stableCloudVersionPath is the URL path that returns the current stable/cloud version.
	stableCloudVersionPath = "/v1/stable/cloud/version"

	// stableCloudCriticalPath is the URL path that returns the stable/cloud critical flag.
	stableCloudCriticalPath = "/v1/stable/cloud/critical"
)

// Version returns the version that should be used for installing Teleport Services
// This is used when installing agents using scripts.
// Even when Teleport Auth/Proxy is using vX, the agents must always respect this version.
func Version(ctx context.Context, versionURL string) (string, error) {
	versionURL, err := getVersionURL(versionURL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionURL, nil)
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

// getVersionURL returns the versionURL or the default stable/cloud version url.
func getVersionURL(versionURL string) (string, error) {
	if versionURL != "" {
		return versionURL, nil
	}
	cloudStableVersionURL, err := url.JoinPath(stableCloudVersionBaseURL, stableCloudVersionPath)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return cloudStableVersionURL, nil
}

// Critical returns true if a critical upgrade is available.
func Critical(ctx context.Context, criticalURL string) (bool, error) {
	criticalURL, err := getCriticalURL(criticalURL)
	if err != nil {
		return false, trace.Wrap(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, criticalURL, nil)
	if err != nil {
		return false, trace.Wrap(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer resp.Body.Close()

	body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return false, trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, trace.BadParameter("invalid status code %d, body: %s", resp.StatusCode, string(body))
	}

	critical := strings.TrimSpace(string(body))

	// The critical endpoint returns either the string "yes" or "no"
	return critical == "yes", nil
}

// getCriticalURL returns the criticalURL or the default stable/cloud critical url.
func getCriticalURL(criticalURL string) (string, error) {
	if criticalURL != "" {
		return criticalURL, nil
	}
	cloudStableCriticalURL, err := url.JoinPath(stableCloudVersionBaseURL, stableCloudCriticalPath)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return cloudStableCriticalURL, nil
}
