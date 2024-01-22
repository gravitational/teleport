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

package cloud

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

type fakeDiscoveryResourceChecker struct {
	fakeError error
}

func (f *fakeDiscoveryResourceChecker) Check(_ context.Context, _ types.Database) error {
	return f.fakeError
}

func Test_discoveryResourceChecker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	successChecker := &fakeDiscoveryResourceChecker{}
	failChecker := &fakeDiscoveryResourceChecker{
		fakeError: trace.BadParameter("check failed"),
	}

	nonCloudDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name: "db",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	cloudDatabase := nonCloudDatabase.Copy()
	cloudDatabase.SetOrigin(types.OriginCloud)

	tests := []struct {
		name          string
		inputCheckers []DiscoveryResourceChecker
		inputDatabase types.Database
		checkError    require.ErrorAssertionFunc
	}{
		{
			name: "success",
			inputCheckers: []DiscoveryResourceChecker{
				successChecker,
				successChecker,
				successChecker,
			},
			inputDatabase: cloudDatabase,
			checkError:    require.NoError,
		},
		{
			name: "fail",
			inputCheckers: []DiscoveryResourceChecker{
				successChecker,
				failChecker,
				successChecker,
			},
			inputDatabase: cloudDatabase,
			checkError:    require.Error,
		},
		{
			name: "skip non-cloud database",
			inputCheckers: []DiscoveryResourceChecker{
				successChecker,
				failChecker,
			},
			inputDatabase: nonCloudDatabase,
			checkError:    require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := discoveryResourceChecker{
				checkers: test.inputCheckers,
			}
			test.checkError(t, c.Check(ctx, test.inputDatabase))
		})
	}
}
