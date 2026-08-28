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

package utils

import (
	"context"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

func TestGetAgentVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	testCases := []struct {
		desc            string
		getter          version.Getter
		expectedVersion *semver.Version
		errorAssert     require.ErrorAssertionFunc
	}{
		{
			desc:            "version getter error",
			getter:          mustStaticGetter(t, "", trace.BadParameter("getter error")),
			expectedVersion: nil,
			errorAssert:     require.Error,
		},
		{
			desc:            "version from getter",
			getter:          mustStaticGetter(t, "1.2.3", nil),
			expectedVersion: semver.Must(version.EnsureSemver("1.2.3")),
			errorAssert:     require.NoError,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			result, err := GetKubeAgentVersion(ctx, tt.getter)
			tt.errorAssert(t, err)
			require.Equal(t, tt.expectedVersion, result)
		})
	}
}

func mustStaticGetter(t *testing.T, v string, err error) version.Getter {
	t.Helper()
	g, gerr := version.NewStaticGetter(v, err)
	require.NoError(t, gerr)
	return g
}
