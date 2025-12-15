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
	"github.com/gravitational/teleport/lib/fixtures"
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

func mustMakeGitHubSSHCA(t *testing.T) types.PluginStaticCredentials {
	t.Helper()
	cred, err := types.NewPluginStaticCredentials(
		types.Metadata{
			Name: uuid.NewString(),
			Labels: map[string]string{
				LabelStaticCredentialsPurpose: PurposeGitHubSSHCA,
			},
		},
		types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_SSHCertAuthorities{
				SSHCertAuthorities: &types.PluginStaticCredentialsSSHCertAuthorities{
					CertAuthorities: []*types.SSHKeyPair{{
						PublicKey:      []byte(fixtures.SSHCAPublicKey),
						PrivateKey:     []byte(fixtures.SSHCAPrivateKey),
						PrivateKeyType: types.PrivateKeyType_RAW,
					}},
				},
			},
		},
	)
	require.NoError(t, err)
	return cred
}

func TestGetByPurpose(t *testing.T) {
	ref := NewRef()
	purpose := "test-found"
	labels := map[string]string{LabelStaticCredentialsPurpose: purpose}
	maps.Copy(labels, ref.Labels)
	cred := mustMakeCred(t, labels)

	tests := []struct {
		name      string
		ref       *types.PluginStaticCredentialsRef
		setupMock func(m *mockByLabelsGetter)
		wantError func(error) bool
		wantCred  types.PluginStaticCredentials
	}{
		{
			name:      "nil ref",
			ref:       nil,
			wantError: trace.IsBadParameter,
		},
		{
			name: "success",
			ref:  ref,
			setupMock: func(m *mockByLabelsGetter) {
				m.On("GetPluginStaticCredentialsByLabels", labels).
					Return([]types.PluginStaticCredentials{cred}, nil)
			},
			wantCred: cred,
		},
		{
			name: "no creds found",
			ref:  ref,
			setupMock: func(m *mockByLabelsGetter) {
				m.On("GetPluginStaticCredentialsByLabels", labels).
					Return([]types.PluginStaticCredentials{}, nil)
			},
			wantError: trace.IsNotFound,
		},
		{
			name: "too many creds found",
			ref:  ref,
			setupMock: func(m *mockByLabelsGetter) {
				m.On("GetPluginStaticCredentialsByLabels", labels).
					Return([]types.PluginStaticCredentials{cred, mustMakeCred(t, labels)}, nil)
			},
			wantError: trace.IsCompareFailed,
		},
		{
			name: "backend issue",
			ref:  ref,
			setupMock: func(m *mockByLabelsGetter) {
				m.On("GetPluginStaticCredentialsByLabels", labels).
					Return(nil, trace.ConnectionProblem(fmt.Errorf("backend"), "problem"))
			},
			wantError: trace.IsConnectionProblem,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := &mockByLabelsGetter{}
			if test.setupMock != nil {
				test.setupMock(m)
			}

			cred, err := GetByPurpose(context.Background(), test.ref, purpose, m)
			if test.wantError != nil {
				require.True(t, test.wantError(err))
				return
			}

			require.NoError(t, err)
			require.Equal(t, test.wantCred, cred)
		})
	}
}

func metadataWithName(name string) types.Metadata {
	return types.Metadata{
		Name: name,
	}
}

func TestGetIntegrationCertAuthorities(t *testing.T) {
	notSupportedIntegration, err := types.NewIntegrationAWSOIDC(
		metadataWithName("not-supported"),
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/OpsTeam",
		},
	)
	require.NoError(t, err)

	githubSpec := &types.GitHubIntegrationSpecV1{
		Organization: "org",
	}
	githubIntegrationNoCreds, err := types.NewIntegrationGitHub(
		metadataWithName("github-no-creds"),
		githubSpec,
	)
	require.NoError(t, err)

	githubIntegration, err := types.NewIntegrationGitHub(
		metadataWithName("github-success"),
		githubSpec,
	)
	require.NoError(t, err)
	githubIntegration.SetCredentials(&types.PluginCredentialsV1{
		Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
			StaticCredentialsRef: NewRef(),
		},
	})

	m := &mockByLabelsGetter{}
	m.On("GetPluginStaticCredentialsByLabels", mock.Anything).
		Return([]types.PluginStaticCredentials{mustMakeGitHubSSHCA(t)}, nil)

	tests := []struct {
		ig           types.Integration
		checkError   func(error) bool
		wantCAKeySet *types.CAKeySet
	}{
		{
			ig:         notSupportedIntegration,
			checkError: trace.IsNotImplemented,
		},
		{
			ig:         githubIntegrationNoCreds,
			checkError: trace.IsBadParameter,
		},
		{
			ig: githubIntegration,
			wantCAKeySet: &types.CAKeySet{
				SSH: []*types.SSHKeyPair{{
					PublicKey:      []byte(fixtures.SSHCAPublicKey),
					PrivateKey:     []byte(fixtures.SSHCAPrivateKey),
					PrivateKeyType: types.PrivateKeyType_RAW,
				}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.ig.GetName(), func(t *testing.T) {
			actualCAKeySet, err := GetIntegrationCertAuthorities(context.Background(), test.ig, m)
			if test.checkError != nil {
				require.True(t, test.checkError(err))
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.wantCAKeySet, actualCAKeySet)
		})
	}
}
