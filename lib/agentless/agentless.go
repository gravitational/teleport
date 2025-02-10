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

// Package agentless provides functions to allow connecting to registered
// OpenSSH (agentless) nodes.
package agentless

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/sshca"
)

// AuthProvider is a subset of the full Auth API that must be connected
// to the root cluster.
type AuthProvider interface {
	GetUser(ctx context.Context, username string, withSecrets bool) (types.User, error)
	GetRole(ctx context.Context, name string) (types.Role, error)
}

// CertGenerator generates certificates from a certificate request. It must
// be connected to the same cluster as the target node that this certificate
// will be generated to authenticate to.
type CertGenerator interface {
	GenerateOpenSSHCert(ctx context.Context, req *proto.OpenSSHCertRequest) (*proto.OpenSSHCert, error)
}

// SignerCreator returns an [ssh.Signer] that can be used to authenticate
// with an agentless node.
type SignerCreator func(ctx context.Context, certGen CertGenerator) (ssh.Signer, error)

// SignerFromSSHIdentity returns a function that attempts to
// create a [ssh.Signer] for the Identity in the provided [ssh.Certificate]
// that is signed with the OpenSSH CA and can be used to authenticate to agentless nodes.
// authClient must be connected to the root cluster, and the CertGenerator
// passed into the returned function must be connected to the same cluster
// as the target node.
func SignerFromSSHIdentity(ident *sshca.Identity, authClient AuthProvider, clusterName, teleportUser string) SignerCreator {
	return func(ctx context.Context, certGen CertGenerator) (ssh.Signer, error) {
		u, err := authClient.GetUser(ctx, teleportUser, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		user, ok := u.(*types.UserV2)
		if !ok {
			return nil, trace.BadParameter("unsupported user type %T", u)
		}

		user.SetRoles(ident.Roles)
		user.SetTraits(ident.Traits)

		// fetch local roles so if the certificate is generated on a leaf
		// cluster it won't have to lookup unknown roles
		roles, err := getRoles(ctx, authClient, ident.Roles)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		validBefore := time.Unix(int64(ident.ValidBefore), 0)
		ttl := time.Until(validBefore)
		params := certParams{
			clusterName:  clusterName,
			teleportUser: user,
			roles:        roles,
			ttl:          ttl,
		}
		signer, err := createAuthSigner(ctx, params, certGen)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return signer, nil
	}
}

// SignerFromAuthzContext returns a function that attempts to
// create a [ssh.Signer] for the [tlsca.Identity] in the provided [authz.Context]
// that is signed with the OpenSSH CA and can be used to authenticate to agentless nodes.
// authClient must be connected to the root cluster, and the CertGenerator
// passed into the returned function must be connected to the same cluster
// as the target node.
func SignerFromAuthzContext(authzCtx *authz.Context, authClient AuthProvider, clusterName string) SignerCreator {
	return func(ctx context.Context, certGen CertGenerator) (ssh.Signer, error) {
		u, ok := authzCtx.User.(*types.UserV2)
		if !ok {
			return nil, trace.BadParameter("unsupported user type %T", u)
		}
		// copy the user to avoid changing it in authzCtx
		user := u.DeepCopy().(*types.UserV2)

		// set the user's roles and traits so impersonation will work correctly
		identity := authzCtx.Identity.GetIdentity()
		user.SetRoles(identity.Groups)
		user.SetTraits(identity.Traits)

		// fetch local roles so if the certificate is generated on a leaf
		// cluster it won't have to lookup unknown roles
		roles, err := getRoles(ctx, authClient, identity.Groups)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		params := certParams{
			clusterName:  clusterName,
			teleportUser: user,
			roles:        roles,
			ttl:          time.Until(identity.Expires),
		}
		signer, err := createAuthSigner(ctx, params, certGen)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return signer, nil
	}
}

func getRoles(ctx context.Context, authClient AuthProvider, roleNames []string) ([]*types.RoleV6, error) {
	roles := make([]*types.RoleV6, len(roleNames))
	for i, roleName := range roleNames {
		r, err := authClient.GetRole(ctx, roleName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		role, ok := r.(*types.RoleV6)
		if !ok {
			return nil, trace.BadParameter("unsupported role type %T", r)
		}
		roles[i] = role
	}

	return roles, nil
}

type certParams struct {
	clusterName  string
	teleportUser *types.UserV2
	roles        []*types.RoleV6
	ttl          time.Duration
}

// createAuthSigner creates a [ssh.Signer] that is signed with
// OpenSSH CA and can be used to authenticate to agentless nodes.
func createAuthSigner(ctx context.Context, params certParams, certGen CertGenerator) (ssh.Signer, error) {
	// generate a new key pair
	priv, err := native.GeneratePrivateKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// sign new public key with OpenSSH CA
	reply, err := certGen.GenerateOpenSSHCert(ctx, &proto.OpenSSHCertRequest{
		User:      params.teleportUser,
		Roles:     params.roles,
		PublicKey: priv.MarshalSSHPublicKey(),
		TTL:       proto.Duration(params.ttl),
		Cluster:   params.clusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// parse returned certificate bytes and create a signer with it
	cert, err := sshutils.ParseCertificate(reply.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privSigner, err := ssh.NewSignerFromSigner(priv.Signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := ssh.NewCertSigner(cert, privSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return signer, nil
}
