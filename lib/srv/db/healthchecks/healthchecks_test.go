/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package healthchecks

import (
	"context"
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

func TestGetHealthChecker(t *testing.T) {
	tests := []struct {
		desc    string
		builder HealthCheckerBuilder
		wantErr string
	}{
		{
			desc:    "valid",
			builder: fakeHealthCheckerBuilder{}.builderFunc(),
		},
		{
			desc:    "builder error",
			builder: fakeHealthCheckerBuilder{builderErr: trace.Errorf("failed to build resolver")}.builderFunc(),
			wantErr: "failed to build resolver",
		},
		{
			desc:    "builder not registered",
			builder: nil,
			wantErr: "is not registered",
		},
	}

	ctx := context.Background()
	for i, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			db := &types.DatabaseV3{}
			db.SetName("dummy")
			db.Spec.Protocol = fmt.Sprintf("fake-%v", i)
			if test.builder != nil {
				RegisterHealthChecker(test.builder, db.Spec.Protocol)
				t.Cleanup(func() {
					healthCheckerBuildersMu.Lock()
					defer healthCheckerBuildersMu.Unlock()
					delete(healthCheckerBuilders, db.Spec.Protocol)
				})
			}

			resolver, err := GetHealthChecker(ctx, HealthCheckerConfig{
				Auth:                  fakeAuth{},
				AuthClient:            &authclient.Client{},
				Clock:                 clockwork.NewFakeClock(),
				Database:              db,
				GCPClients:            fakeGCPClients{},
				UpdateProxiedDatabase: func(string, func(types.Database) error) error { return nil },
			})
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				require.Nil(t, resolver)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resolver)
		})
	}
}

type fakeHealthCheckerBuilder struct {
	builderErr error
}

func (f fakeHealthCheckerBuilder) builderFunc() HealthCheckerBuilder {
	return func(context.Context, HealthCheckerConfig) (healthcheck.HealthChecker, error) {
		if f.builderErr != nil {
			return nil, trace.Wrap(f.builderErr)
		}
		return healthcheck.NewTargetDialer(func(ctx context.Context) ([]string, error) {
			return nil, nil
		}), nil
	}
}

type fakeAuth struct {
	common.Auth
}

type fakeGCPClients struct {
	cloud.GCPClients
}
