/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/teleport/e2e/runner/fixtures"
	"github.com/moby/moby/client"
)

type nodeArch string

const (
	amd64 nodeArch = "amd64"
	arm64 nodeArch = "arm64"
)

// crossCompilers maps a GOARCH to the C cross-compiler used to build the docker node binary
var crossCompilers = map[nodeArch]string{
	amd64: "x86_64-unknown-linux-gnu-gcc",
	arm64: "aarch64-unknown-linux-gnu-gcc",
}

// resolveNodeArch returns the architecture to build and run the docker node for
func resolveNodeArch(ctx context.Context) (nodeArch, error) {
	if !fixtures.SSHNodeBPF.Enabled {
		return amd64, nil
	}

	arch, err := daemonArch(ctx)
	if err != nil {
		return "", fmt.Errorf("determining node architecture for enhanced recording: %w", err)
	}

	return arch, nil
}

// unameToGOARCH maps the machine names the docker daemon reports to their GOARCH equivalents.
var unameToGOARCH = map[string]nodeArch{
	"x86_64":  amd64,
	"amd64":   amd64,
	"aarch64": arm64,
	"arm64":   arm64,
}

// daemonArch returns the GOARCH of the machine running the docker daemon, which is not necessarily
// the machine running the tests.
func daemonArch(ctx context.Context) (nodeArch, error) {
	api, err := dockerAPI()
	if err != nil {
		return "", err
	}

	result, err := api.Info(ctx, client.InfoOptions{})
	if err != nil {
		return "", fmt.Errorf("querying docker daemon info: %w", err)
	}

	arch, ok := unameToGOARCH[strings.ToLower(result.Info.Architecture)]
	if !ok {
		return "", fmt.Errorf("unsupported docker daemon architecture %q", result.Info.Architecture)
	}

	return arch, nil
}
