// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package clientversion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/utils"
)

// ErrClientTooOld indicates this client is older than the cluster's minimum
// client version.
var ErrClientTooOld = errors.New("this client is older than the minimum supported version required by the cluster and will not be able to connect until it is upgraded")

// Config configures [Check].
type Config struct {
	// ProxyAddr is the address of the proxy.
	ProxyAddr string
	// Insecure skips verification of the proxy's certificate.
	Insecure bool
	// Skip bypasses the check with a warning when the client is too old.
	Skip bool
	// Log is used for the fail-open and skip warnings. It defaults to
	// [slog.Default] when nil.
	Log *slog.Logger
	// Testing is a group of properties that are used in tests.
	Testing TestingConfig
}

// TestingConfig holds overrides used only for testing purposes.
type TestingConfig struct {
	// ClientVersion is used to override the local version in tests.
	ClientVersion string
}

// Check fetches the cluster's advertised minimum client version from the
// proxy and returns [ErrClientTooOld] when the local version is below it. It is
// best-effort and fails open, returning nil when the minimum can't be fetched
// or parsed. It is bypassed with a warning when too old and [Config.Skip] is set.
func Check(ctx context.Context, cfg Config) error {
	log := cfg.Log
	if log == nil {
		log = slog.Default()
	}

	resp, err := webclient.Find(&webclient.Config{
		Context:   ctx,
		ProxyAddr: cfg.ProxyAddr,
		Insecure:  cfg.Insecure,
	})
	if err != nil {
		log.WarnContext(ctx,
			"Could not fetch the cluster's minimum client version from the proxy's web API, skipping check.",
			"proxy_addr", cfg.ProxyAddr,
			"error", err,
		)
		return nil
	}

	localVersion := teleport.Version
	if cfg.Testing.ClientVersion != "" {
		localVersion = cfg.Testing.ClientVersion
	}

	err = meetsMinVersion(localVersion, resp.MinClientVersion)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrClientTooOld) {
		log.WarnContext(ctx,
			"Could not parse the cluster's minimum client version, skipping check.",
			"error", err,
			"min_client_version", resp.MinClientVersion,
		)
		return nil
	}
	if cfg.Skip {
		log.WarnContext(ctx,
			"This client is older than the cluster's minimum supported version, ignoring the version check because --skip-version-check is set.",
			"error", err,
		)
		return nil
	}
	return trace.Wrap(err)
}

// meetsMinVersion returns [ErrClientTooOld] when clientVersion is below
// minVersion. An empty minimum is treated as no constraint; a malformed minimum
// returns a parse error that is not [ErrClientTooOld] so callers can fail open.
func meetsMinVersion(clientVersion, minVersion string) error {
	if minVersion == "" {
		return nil
	}
	// Stripping the pre-release both validates the advertised minimum and yields a
	// clean version for the user-facing error message.
	minWithoutPreRelease, err := utils.VersionWithoutPreRelease(minVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	if !utils.MeetsMinVersion(clientVersion, minVersion) {
		return fmt.Errorf("%w (client v%s, minimum v%s)", ErrClientTooOld, clientVersion, minWithoutPreRelease)
	}
	return nil
}
