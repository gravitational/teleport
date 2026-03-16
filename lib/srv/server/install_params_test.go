/*
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

package server

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	autoupdate "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestCheckInstallParamsManagedUpdates(t *testing.T) {
	withSuffix := &types.InstallerParams{Suffix: "test-suffix"}
	withUpdateGroup := &types.InstallerParams{UpdateGroup: "staging"}
	withBoth := &types.InstallerParams{Suffix: "test-suffix", UpdateGroup: "staging"}
	withoutManagedUpdatesParams := &types.InstallerParams{}

	tests := []struct {
		name       string
		params     *types.InstallerParams
		rolloutErr error
		errCheck   require.ErrorAssertionFunc
	}{
		{
			name:       "no managed-updates params, no error regardless of rollout",
			params:     withoutManagedUpdatesParams,
			rolloutErr: trace.NotFound("not found"),
			errCheck:   require.NoError,
		},
		{
			name:       "suffix present, rollout exists: no error",
			params:     withSuffix,
			rolloutErr: nil,
			errCheck:   require.NoError,
		},
		{
			name:       "suffix present, rollout not found: error",
			params:     withSuffix,
			rolloutErr: trace.NotFound("not found"),
			errCheck:   require.Error,
		},
		{
			name:       "suffix present, unexpected rollout error: error",
			params:     withSuffix,
			rolloutErr: trace.ConnectionProblem(nil, "connection failed"),
			errCheck:   require.Error,
		},
		{
			name:       "update_group present, rollout not found: error",
			params:     withUpdateGroup,
			rolloutErr: trace.NotFound("not found"),
			errCheck:   require.Error,
		},
		{
			name:       "update_group present, rollout exists: no error",
			params:     withUpdateGroup,
			rolloutErr: nil,
			errCheck:   require.NoError,
		},
		{
			name:       "both suffix and update_group present, rollout not found: error",
			params:     withBoth,
			rolloutErr: trace.NotFound("not found"),
			errCheck:   require.Error,
		},
		{
			name:       "empty params: no error",
			params:     nil,
			rolloutErr: trace.NotFound("not found"),
			errCheck:   require.NoError,
		},
		{
			name:       "suffix present, old auth server (not implemented): error",
			params:     withSuffix,
			rolloutErr: trace.NotImplemented("not implemented"),
			errCheck:   require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getter := &mockRolloutGetter{err: tt.rolloutErr}
			err := CheckInstallParamsManagedUpdates(t.Context(), tt.params, getter)
			tt.errCheck(t, err)
		})
	}
}

type mockRolloutGetter struct {
	err error
}

func (f *mockRolloutGetter) GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdate.AutoUpdateAgentRollout, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &autoupdate.AutoUpdateAgentRollout{}, nil
}
