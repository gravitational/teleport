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

package azure

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	cloudazure "github.com/gravitational/teleport/lib/cloud/azure"
)

type fakeTokenCredential struct {
	lastSeenScope string
}

func (f *fakeTokenCredential) GetToken(_ context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	if len(opts.Scopes) != 1 {
		return azcore.AccessToken{}, trace.BadParameter("expect one scope but got %v", opts.Scopes)
	}

	f.lastSeenScope = opts.Scopes[0]
	return azcore.AccessToken{
		Token:     "fake-token",
		ExpiresOn: time.Now().Add(time.Hour),
	}, nil
}

type fakeCredentialProvider struct {
	cred             fakeTokenCredential
	lastSeenIdentity string
}

func (f *fakeCredentialProvider) MakeCredential(_ context.Context, userRequestedIdentity string) (azcore.TokenCredential, error) {
	f.lastSeenIdentity = userRequestedIdentity
	return &f.cred, nil
}

func (f *fakeCredentialProvider) MapScope(scope string) string {
	return scope + ".mapped"
}

func Test_getAccessTokenFromCredentialProvider(t *testing.T) {
	fakeCredProvider := &fakeCredentialProvider{}
	userRequestedIdentity := "/subscriptions/my-sub/resourcegroups/my-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/my-name"
	ctx := context.Background()

	token, err := getAccessTokenFromCredentialProvider(fakeCredProvider)(ctx, userRequestedIdentity, "test-scope")
	require.NoError(t, err)
	require.Equal(t, "fake-token", token.Token)
	require.Equal(t, userRequestedIdentity, fakeCredProvider.lastSeenIdentity)
	require.Equal(t, "test-scope.mapped", fakeCredProvider.cred.lastSeenScope)
}

func Test_workloadIdentityCredentialProvider(t *testing.T) {
	ctx := context.Background()
	fakeAgentIdentity := &fakeTokenCredential{}
	credProvider, err := newWorloadIdentityCredentialProvider(ctx, fakeAgentIdentity)
	require.NoError(t, err)

	// Hook up more mocks.
	fakeWorkloadIdentityCredential := &fakeTokenCredential{}
	userRequestedIdentity := cloudazure.NewUserAssignedIdentity("my-sub", "my-group", "my-name", "my-client-id")
	mockAPI := cloudazure.NewARMUserAssignedIdentitiesMock(userRequestedIdentity)
	credProvider.newClient = func(string, azcore.TokenCredential, *arm.ClientOptions) (*cloudazure.UserAssignedIdentitiesClient, error) {
		return cloudazure.NewUserAssignedIdentitiesClientByAPI(mockAPI), nil
	}
	credProvider.newCredential = func(clientID string) (azcore.TokenCredential, error) {
		if clientID != "my-client-id" {
			return nil, trace.BadParameter("expect my-client-id but got %s", clientID)
		}
		return fakeWorkloadIdentityCredential, nil
	}

	t.Run("MakeCredential", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			actualCredential, err := credProvider.MakeCredential(ctx, *userRequestedIdentity.ID)
			require.NoError(t, err)
			require.Same(t, fakeWorkloadIdentityCredential, actualCredential)
		})
		t.Run("fail to get client ID", func(t *testing.T) {
			notFoundIdentity := "/subscriptions/my-sub/resourcegroups/my-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/not-my-name"
			_, err := credProvider.MakeCredential(ctx, notFoundIdentity)
			require.Error(t, err)
		})
	})

	t.Run("MapScope", func(t *testing.T) {
		tests := []struct {
			inputScope  string
			outputScope string
		}{
			{
				inputScope:  "https://management.core.windows.net/",
				outputScope: "https://management.core.windows.net/.default",
			},
			{
				inputScope:  "some-other-scope",
				outputScope: "some-other-scope",
			},
		}
		for _, test := range tests {
			t.Run(test.inputScope, func(t *testing.T) {
				require.Equal(t, test.outputScope, credProvider.MapScope(test.inputScope))
			})
		}
	})
}
