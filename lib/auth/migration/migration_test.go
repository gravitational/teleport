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
	"github.com/gravitational/teleport/lib/utils"
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
	log := utils.NewSlogLoggerForTests()
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

			test.errAssertion(t, Apply(ctx, log, b, withMigrations(test.migrations)))

			current, err := getCurrentMigration(ctx, b)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(&test.expectedStatus, current, cmpopts.IgnoreFields(migrationStatus{}, "Started", "Completed")))
		})
	}
}
