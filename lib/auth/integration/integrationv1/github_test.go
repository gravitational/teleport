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

package integrationv1

import (
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/durationpb"

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/modules"
)

func TestGenerateGitHubUserCert(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

	ca := newCertAuthority(t, types.HostCA, "test-cluster")
	ctx, _, resourceSvc := initSvc(t, ca, ca.GetClusterName(), "127.0.0.1.nip.io")

	githubIntegration, err := newGitHubIntegration("github-my-org", "id", "secret")
	require.NoError(t, err)

	adminCtx := authz.ContextWithUser(ctx, authz.BuiltinRole{
		Role:     types.RoleAdmin,
		Username: string(types.RoleAdmin),
	})
	_, err = resourceSvc.CreateIntegration(adminCtx, &integrationpb.CreateIntegrationRequest{Integration: githubIntegration})
	require.NoError(t, err)

	key, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)

	req := &integrationpb.GenerateGitHubUserCertRequest{
		Integration: "github-my-org",
		PublicKey:   key.MarshalSSHPublicKey(),
		UserId:      "1122334455",
		KeyId:       "alice",
		Ttl:         durationpb.New(time.Minute),
	}

	// Admin users cannot generate certs.
	_, err = resourceSvc.GenerateGitHubUserCert(adminCtx, req)
	require.True(t, trace.IsAccessDenied(err))

	// Call as Proxy.
	proxyCtx := authz.ContextWithUser(ctx, authz.BuiltinRole{
		Role:     types.RoleProxy,
		Username: string(types.RoleProxy),
	})
	resp, err := resourceSvc.GenerateGitHubUserCert(proxyCtx, req)
	require.NoError(t, err)
	authorizedKey, _, _, _, err := ssh.ParseAuthorizedKey(resp.AuthorizedKey)
	require.NoError(t, err)
	sshCert, ok := authorizedKey.(*ssh.Certificate)
	require.True(t, ok)

	assert.Equal(t, "alice", sshCert.KeyId)
	assert.Equal(t, map[string]string{"id@github.com": "1122334455"}, sshCert.Permissions.Extensions)
}
