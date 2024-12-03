/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package credentials

import (
	"context"
	"fmt"
	"maps"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type mockByLabelsGetter struct {
	mock.Mock
}

func (m *mockByLabelsGetter) GetPluginStaticCredentialsByLabels(_ context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error) {
	args := m.Called(labels)
	creds, ok := args.Get(0).([]types.PluginStaticCredentials)
	if ok {
		return creds, args.Error(1)
	}
	return nil, args.Error(1)
}

func mustMakeCred(t *testing.T, labels map[string]string) types.PluginStaticCredentials {
	t.Helper()
	cred, err := types.NewPluginStaticCredentials(
		types.Metadata{
			Name:   uuid.NewString(),
			Labels: labels,
		},
		types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: "token",
			},
		},
	)
	require.NoError(t, err)
	return cred
}

func TestGetByPurpose(t *testing.T) {
	ref := NewRef()

	wantPurpose := "test-found"
	notFoundPurpose := "test-not-found"
	backendIssuePurpose := "test-backend-issue"

	wantLabels := map[string]string{LabelStaticCredentialsPurpose: wantPurpose}
	maps.Copy(wantLabels, ref.Labels)
	wantCred := mustMakeCred(t, wantLabels)

	notFoundLabels := map[string]string{LabelStaticCredentialsPurpose: notFoundPurpose}
	maps.Copy(notFoundLabels, ref.Labels)

	m := &mockByLabelsGetter{}
	m.On("GetPluginStaticCredentialsByLabels", wantLabels).Return([]types.PluginStaticCredentials{wantCred}, nil)
	m.On("GetPluginStaticCredentialsByLabels", notFoundLabels).Return([]types.PluginStaticCredentials{}, nil)
	m.On("GetPluginStaticCredentialsByLabels", mock.Anything).Return(nil, trace.ConnectionProblem(fmt.Errorf("backend error"), "backend error"))

	tests := []struct {
		name      string
		ref       *types.PluginStaticCredentialsRef
		purpose   string
		wantError func(error) bool
		wantCred  types.PluginStaticCredentials
	}{
		{
			name:      "nil ref",
			ref:       nil,
			purpose:   wantPurpose,
			wantError: trace.IsBadParameter,
		},
		{
			name:     "success",
			ref:      ref,
			purpose:  wantPurpose,
			wantCred: wantCred,
		},
		{
			name:      "no creds found",
			ref:       ref,
			purpose:   notFoundPurpose,
			wantError: trace.IsNotFound,
		},
		{
			name:      "backend issue",
			ref:       ref,
			purpose:   backendIssuePurpose,
			wantError: trace.IsConnectionProblem,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cred, err := GetByPurpose(context.Background(), test.ref, test.purpose, m)
			if test.wantError != nil {
				require.True(t, test.wantError(err))
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.wantCred, cred)
		})
	}
}
