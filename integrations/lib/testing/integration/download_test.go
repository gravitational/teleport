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

package integration

import (
	"context"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/lib"
)

//go:embed download_sha.dsv_1204
var downloadVersionsDSV1204 string

func TestDownloadVersionsHash(t *testing.T) {
	ctx := context.Background()
	dv, ok := downloadVersionsHash(ctx, downloadVersionsDSV1204, downloadVersionKey{
		ver:        "v12.0.4",
		os:         "linux",
		arch:       "amd64",
		enterprise: false,
	})
	require.True(t, ok, "expected to find hash for key, but didn't")
	require.Equal(t, dv.sha256, lib.MustHexSHA256("84ce1cd7297499e6b52acf63b1334890abc39c926c7fc2d0fe676103d200752a"))

	dv, ok = downloadVersionsHash(ctx, downloadVersionsDSV1204, downloadVersionKey{
		ver:        "v12.0.4",
		os:         "linux",
		arch:       "amd64",
		enterprise: true,
	})
	require.True(t, ok, "expected to find hash for key, but didn't")
	require.Equal(t, dv.sha256, lib.MustHexSHA256("b27c3e16ce264e33feabda7e22eaa7917f585c28bf2eb31f60944f9e961aa7a8"))
}
