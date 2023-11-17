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
