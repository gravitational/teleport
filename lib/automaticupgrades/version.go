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

	resp, err := sendRequest(ctx, versionURL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return resp, nil
}

// Critical returns true if a critical upgrade is available.
func Critical(ctx context.Context, criticalURL string) (bool, error) {
	criticalURL, err := getCriticalURL(criticalURL)
	if err != nil {
		return false, trace.Wrap(err)
	}

	critical, err := sendRequest(ctx, criticalURL)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// Expects critical endpoint to return either the string "yes" or "no"
	switch critical {
	case "yes":
		return true, nil
	case "no":
		return false, nil
	default:
		return false, trace.BadParameter("critical endpoint returned an unexpected value: %v", critical)
	}
}

// sendRequest sends a GET request to the reqURL and returns the response value
func sendRequest(ctx context.Context, reqURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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

	return strings.TrimSpace(string(body)), trace.Wrap(err)
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
