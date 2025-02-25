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

package endpoint

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/gravitational/trace"

	versionlib "github.com/gravitational/teleport/lib/automaticupgrades/version"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/versioncontrol"
)

const stableCloudPath = "v1/webapi/automaticupgrades/channel/stable/cloud"

// Export exports the proxy version server config.
func Export(ctx context.Context, proxyAddr string) error {
	versionEndpoint := fmt.Sprint(path.Join(proxyAddr, stableCloudPath))
	if err := verifyVersionEndpoint(ctx, versionEndpoint); err != nil {
		return trace.Wrap(err, "version endpoint may be invalid or unreachable")
	}

	appliedEndpoint, err := exportEndpoint(versioncontrol.UnitConfigDir, versionEndpoint)
	if err != nil {
		return trace.Wrap(err, "failed to export version endpoint")
	}

	if err := verifyVersionEndpoint(ctx, appliedEndpoint); err != nil {
		return trace.Wrap(err, "applied version endpoint may be invalid or unreachable")
	}

	return nil
}

// verifyVersionEndpoint verifies that the provided endpoint serves a valid teleport
// version.
func verifyVersionEndpoint(ctx context.Context, endpoint string) error {
	baseURL, err := url.Parse(fmt.Sprintf("https://%s", endpoint))
	if err != nil {
		return trace.Wrap(err)
	}
	versionGetter := versionlib.NewBasicHTTPVersionGetter(baseURL)
	_, err = versionGetter.GetVersion(ctx)
	return trace.Wrap(err)
}

// exportEndpoint exports the versionEndpoint to the specified config directory.
// If an existing value is already present, it will not be overwritten. The resulting
// version endpoint value will be returned.
func exportEndpoint(configDir, versionEndpoint string) (string, error) {
	// ensure config dir exists. if created it is set to 755, which is reasonably safe and seems to
	// be the standard choice for config dirs like this in /etc/.
	if err := os.MkdirAll(configDir, defaults.DirectoryPermissions); err != nil {
		return "", trace.Wrap(err)
	}

	// open/create endpoint file. if created it is set to 644, which is reasonable for a sensitive but non-secret config value.
	endpointFile, err := os.OpenFile(path.Join(configDir, "endpoint"), os.O_RDWR|os.O_CREATE, defaults.FilePermissions)
	if err != nil {
		return "", trace.Wrap(err, "failed to open endpoint config file")
	}
	defer endpointFile.Close()

	b, err := io.ReadAll(endpointFile)
	if err != nil {
		return "", trace.Wrap(err, "failed to read endpoint config file")
	}

	// Do not overwrite if an endpoint value is already configured.
	if len(b) != 0 {
		return strings.TrimSuffix(string(b), "\n"), nil
	}

	_, err = endpointFile.Write([]byte(versionEndpoint))
	if err != nil {
		return "", trace.Wrap(err, "failed to write endpoint config file")
	}
	return versionEndpoint, nil
}
