// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package migration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

type testMigration struct {
	version int64
	name    string
	up      func(ctx context.Context, b backend.Backend) error
	down    func(ctx context.Context, b backend.Backend) error
}

func (t testMigration) Up(ctx context.Context, b backend.Backend) error {
	if t.up == nil {
		return nil
	}
	return t.up(ctx, b)
}

func (t testMigration) Down(ctx context.Context, b backend.Backend) error {
	if t.down == nil {
		return nil
	}

	return t.down(ctx, b)
}

func (t testMigration) Version() int64 {
	return t.version
}

func (t testMigration) Name() string {
	return t.name
}

func TestApply(t *testing.T) {
	cases := []struct {
		name           string
		migrations     []migration
		initialStatus  *migrationStatus
		expectedStatus migrationStatus
		errAssertion   require.ErrorAssertionFunc
	}{
		{
			name: "migrations up to date",
			migrations: []migration{
				testMigration{version: 1},
				testMigration{version: 2},
				testMigration{version: 3},
			},
			initialStatus: &migrationStatus{
				Version: 3,
				Phase:   migrationPhaseComplete,
			},
			expectedStatus: migrationStatus{
				Version: 3,
				Phase:   migrationPhaseComplete,
			},
			errAssertion: require.NoError,
		},
		{
			name: "deleted migrations",
			initialStatus: &migrationStatus{
				Version: 2,
				Phase:   migrationPhaseComplete,
			},
			errAssertion: require.Error,
			expectedStatus: migrationStatus{
				Version: 2,
				Phase:   migrationPhaseComplete,
			},
		},
		{
			name: "previous migration attempt failed",
			initialStatus: &migrationStatus{
				Version: 2,
				Phase:   migrationPhaseError,
			},
			migrations: []migration{
				testMigration{version: 1},
				testMigration{version: 2},
				testMigration{version: 3},
			},
			errAssertion: require.Error,
			expectedStatus: migrationStatus{
				Version: 2,
				Phase:   migrationPhaseError,
			},
		},
		{
			name: "missing migration",
			initialStatus: &migrationStatus{
				Version: 0,
				Phase:   migrationPhaseComplete,
			},
			migrations: []migration{
				testMigration{version: 1},
				testMigration{version: 3},
			},
			errAssertion: require.Error,
			expectedStatus: migrationStatus{
				Version: 1,
				Phase:   migrationPhaseComplete,
			},
		},
		{
			name: "migration failed",
			migrations: []migration{
				testMigration{version: 1},
				testMigration{version: 2},
				testMigration{version: 3, up: func(ctx context.Context, b backend.Backend) error {
					return errors.New("failure")
				}},
				testMigration{version: 4},
				testMigration{version: 5},
			},
			errAssertion: require.Error,
			expectedStatus: migrationStatus{
				Version: 3,
				Phase:   migrationPhaseError,
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			b, err := memory.New(memory.Config{EventsOff: true})
			require.NoError(t, err)

			ctx := context.Background()

			if test.initialStatus != nil {
				require.NoError(t, setCurrentMigration(ctx, b, *test.initialStatus))
			}

			test.errAssertion(t, Apply(ctx, b, withMigrations(test.migrations)))

			current, err := getCurrentMigration(ctx, b)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(&test.expectedStatus, current, cmpopts.IgnoreFields(migrationStatus{}, "Started", "Completed")))
		})
	}
}
