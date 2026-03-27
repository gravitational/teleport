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
	"google.golang.org/protobuf/proto"

	authproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptosuites"
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
	GenerateOpenSSHCert(ctx context.Context, req *authproto.OpenSSHCertRequest) (*authproto.OpenSSHCert, error)
}

// LocalAccessPoint should be a cache of the local cluster auth preference.
type LocalAccessPoint interface {
	GetAuthPreference(context.Context) (types.AuthPreference, error)
}

// SignerCreator returns an [ssh.Signer] that can be used to authenticate
// with an agentless node. localAccessPoint is a cache of the local cluster
// auth preference. login is the OS user for the session (used for scoped access).
type SignerCreator func(ctx context.Context, localAccessPoint LocalAccessPoint, login string) (ssh.Signer, error)

// SignerFromSSHIdentity returns a function that attempts to
// create a [ssh.Signer] for the Identity in the provided [ssh.Certificate]
// that is signed with the OpenSSH CA and can be used to authenticate to agentless nodes.
// authClient must be connected to the root cluster. certGen must be connected
// to the same cluster as the target node.
func SignerFromSSHIdentity(ident *sshca.Identity, authClient AuthProvider, certGen CertGenerator, clusterName, teleportUser string) SignerCreator {
	return func(ctx context.Context, localAccessPoint LocalAccessPoint, login string) (ssh.Signer, error) {
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

		var scopePinBytes []byte
		if ident.ScopePin != nil {
			scopePinBytes, err = proto.Marshal(ident.ScopePin)
			if err != nil {
				return nil, trace.Wrap(err, "marshaling scope pin")
			}
		}

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
			login:        login,
			scopePin:     scopePinBytes,
		}
		signer, err := createAuthSigner(ctx, params, localAccessPoint, certGen)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return signer, nil
	}
}

// SignerFromAuthzContext returns a function that attempts to
// create a [ssh.Signer] for the [tlsca.Identity] in the provided [authz.Context]
// that is signed with the OpenSSH CA and can be used to authenticate to agentless nodes.
// authClient must be connected to the root cluster. certGen must be connected
// to the same cluster as the target node.
func SignerFromAuthzContext(authzCtx *authz.Context, authClient AuthProvider, certGen CertGenerator, clusterName string) SignerCreator {
	return signerFromIdentity(authzCtx.User, authzCtx.Identity, authClient, certGen, clusterName)
}

// SignerFromScopedContext returns a function that attempts to create a [ssh.Signer]
// for a scoped identity that is signed with the OpenSSH CA and can be used to authenticate to agentless nodes.
// authClient must be connected to the root cluster. certGen must be connected
// to the same cluster as the target node.
// The login is provided lazily when the SignerCreator is called (after the
// SSH handshake reveals the target OS user).
func SignerFromScopedContext(authzCtx *authz.ScopedContext, authClient AuthProvider, certGen CertGenerator, clusterName string) SignerCreator {
	return signerFromIdentity(authzCtx.User, authzCtx.Identity, authClient, certGen, clusterName)
}

func signerFromIdentity(user types.User, identityGetter authz.IdentityGetter, authClient AuthProvider, certGen CertGenerator, clusterName string) SignerCreator {
	return func(ctx context.Context, localAccessPoint LocalAccessPoint, login string) (ssh.Signer, error) {
		u, ok := user.(*types.UserV2)
		if !ok {
			return nil, trace.BadParameter("unsupported user type %T", u)
		}
		// copy the user to avoid mutating the original
		userCopy := u.DeepCopy().(*types.UserV2)

		// set the user's roles and traits so impersonation will work correctly
		identity := identityGetter.GetIdentity()
		userCopy.SetRoles(identity.Groups)
		userCopy.SetTraits(identity.Traits)

		// fetch local roles so if the certificate is generated on a leaf
		// cluster it won't have to lookup unknown roles
		roles, err := getRoles(ctx, authClient, identity.Groups)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Marshal the scope pin from the user's certificate if present, so it can
		// be propagated to the auth server for building the scoped access checker.
		var scopePinBytes []byte
		if identity.ScopePin != nil {
			scopePinBytes, err = proto.Marshal(identity.ScopePin)
			if err != nil {
				return nil, trace.Wrap(err, "marshaling scope pin")
			}

			if login == "" {
				return nil, trace.AccessDenied("login required for generating certs for agentless nodes")
			}
		}

		params := certParams{
			clusterName:  clusterName,
			teleportUser: userCopy,
			roles:        roles,
			ttl:          time.Until(identity.Expires),
			scopePin:     scopePinBytes,
			login:        login,
		}
		signer, err := createAuthSigner(ctx, params, localAccessPoint, certGen)
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
	scopePin     []byte
	login        string
}

// createAuthSigner creates a [ssh.Signer] that is signed with
// OpenSSH CA and can be used to authenticate to agentless nodes.
func createAuthSigner(ctx context.Context, params certParams, localAccessPoint LocalAccessPoint, certGen CertGenerator) (ssh.Signer, error) {
	// generate a new key pair
	key, err := cryptosuites.GenerateKey(ctx, cryptosuites.GetCurrentSuiteFromAuthPreference(localAccessPoint), cryptosuites.UserSSH)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	priv, err := keys.NewPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// sign new public key with OpenSSH CA
	req := &authproto.OpenSSHCertRequest{
		User:      params.teleportUser,
		Roles:     params.roles,
		PublicKey: priv.MarshalSSHPublicKey(),
		TTL:       authproto.Duration(params.ttl),
		Cluster:   params.clusterName,
		ScopePin:  params.scopePin,
		Login:     params.login,
	}

	reply, err := certGen.GenerateOpenSSHCert(ctx, req)
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
