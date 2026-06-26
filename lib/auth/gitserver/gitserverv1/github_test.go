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
package gitserverv1

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	"github.com/gravitational/teleport/api/types"
)

// TestCreateGitHubAuthRequest verifies the output from the
// CreateGitHubAuthRequest API. Note that RBAC of this API is tested in
// TestServiceAccess.
func TestCreateGitHubAuthRequest(t *testing.T) {
	ctx := context.Background()
	org1 := newServer(t, "org1")

	checker := &fakeAccessChecker{
		allowVerbs:    []string{types.VerbRead, types.VerbList},
		allowResource: true,
	}

	service := newService(t, checker, org1)
	createdRequest, err := service.CreateGitHubAuthRequest(ctx, &pb.CreateGitHubAuthRequestRequest{
		Request:      &types.GithubAuthRequest{},
		Organization: org1.GetGitHub().Organization,
	})
	require.NoError(t, err)
	require.NotNil(t, createdRequest)

	wantedRequest := &types.GithubAuthRequest{
		CertTTL: time.Hour,
		ConnectorSpec: &types.GithubConnectorSpecV3{
			ClientID:       fakeClientID,
			ClientSecret:   fakeClientSecret,
			RedirectURL:    fmt.Sprintf("https://%s/v1/webapi/github/callback", fakeProxyAddr),
			EndpointURL:    "https://github.com",
			APIEndpointURL: "https://api.github.com",
		},
		AuthenticatedUser: fakeTeleportUser,
	}
	require.Empty(t, cmp.Diff(createdRequest, wantedRequest,
		cmpopts.IgnoreTypes([]types.TeamRolesMapping{}),
		cmpopts.IgnoreFields(types.GithubAuthRequest{}, "ConnectorID"),
		cmpopts.EquateEmpty(),
	))
}

func TestCreateGitHubAuthRequest_CallbackURL(t *testing.T) {
	ctx := context.Background()
	org1 := newServer(t, "org1")

	checker := &fakeAccessChecker{
		allowVerbs:    []string{types.VerbRead, types.VerbList},
		allowResource: true,
	}

	tests := []struct {
		name            string
		allowProtocols  []string
		wantRedirectURL string
	}{
		{
			name:            "SSH-only uses legacy callback",
			allowProtocols:  []string{types.GitProtocolSSH},
			wantRedirectURL: fmt.Sprintf("https://%s/v1/webapi/github/callback", fakeProxyAddr),
		},
		{
			name:            "default (nil) uses legacy callback",
			wantRedirectURL: fmt.Sprintf("https://%s/v1/webapi/github/callback", fakeProxyAddr),
		},
		{
			name:            "HTTP enabled uses authenticated callback",
			allowProtocols:  []string{types.GitProtocolHTTP},
			wantRedirectURL: fmt.Sprintf("https://%s/web/github/integration/callback", fakeProxyAddr),
		},
		{
			name:            "both protocols uses authenticated callback",
			allowProtocols:  []string{types.GitProtocolSSH, types.GitProtocolHTTP},
			wantRedirectURL: fmt.Sprintf("https://%s/web/github/integration/callback", fakeProxyAddr),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newServiceWithAllowProtocols(t, checker, test.allowProtocols, org1)
			createdRequest, err := service.CreateGitHubAuthRequest(ctx, &pb.CreateGitHubAuthRequestRequest{
				Request:      &types.GithubAuthRequest{},
				Organization: org1.GetGitHub().Organization,
			})
			require.NoError(t, err)
			require.Equal(t, test.wantRedirectURL, createdRequest.ConnectorSpec.RedirectURL)
		})
	}
}
