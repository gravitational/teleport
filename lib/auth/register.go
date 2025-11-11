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

package auth

import (
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
)

// LocalRegister is used to generate host keys when a node or proxy is running
// within the same process as the Auth Server and as such, does not need to
// use provisioning tokens.
func LocalRegister(id state.IdentityID, authServer *Server, additionalPrincipals, dnsNames []string, remoteAddr string, systemRoles []types.SystemRole) (*state.Identity, error) {
	key, err := cryptosuites.GenerateKey(context.Background(), cryptosuites.GetCurrentSuiteFromAuthPreference(authServer), cryptosuites.HostIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshPub, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsPub, err := keys.MarshalPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If local registration is happening and no remote address was passed in
	// (which means no advertise IP was set), use localhost. This behavior must
	// be kept consistent with the equivalen behavior in cert rotation/re-register
	// logic in lib/service.
	if remoteAddr == "" {
		remoteAddr = defaults.Localhost
	}
	certs, err := authServer.GenerateHostCerts(context.Background(),
		&proto.HostCertsRequest{
			HostID:               id.HostUUID,
			NodeName:             id.NodeName,
			Role:                 id.Role,
			AdditionalPrincipals: additionalPrincipals,
			RemoteAddr:           remoteAddr,
			DNSNames:             dnsNames,
			NoCache:              true,
			PublicSSHKey:         ssh.MarshalAuthorizedKey(sshPub),
			PublicTLSKey:         tlsPub,
			SystemRoles:          systemRoles,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	priv, err := keys.MarshalPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := state.ReadIdentityFromKeyPair(priv, certs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return identity, nil
}

// ReRegisterParams specifies parameters for re-registering
// in the cluster (rotating certificates for existing members)
type ReRegisterParams struct {
	// Client is an authenticated client using old credentials
	Client ReRegisterClient
	// ID is identity ID
	ID state.IdentityID
	// AdditionalPrincipals is a list of additional principals to dial
	AdditionalPrincipals []string
	// DNSNames is a list of DNS Names to add to the x509 client certificate
	DNSNames []string
	// RemoteAddr overrides the RemoteAddr host cert generation option when
	// performing re-registration locally (this value has no effect for remote
	// registration and can be omitted).
	RemoteAddr string
	// Rotation is the rotation state of the certificate authority
	Rotation types.Rotation
	// SystemRoles is a set of additional system roles held by the instance.
	SystemRoles []types.SystemRole
	// Used by older instances to requisition a multi-role cert by individually
	// proving which system roles are held.
	SystemRoleAssertionID string
}

// ReRegisterClient abstracts over local auth servers and remote clients when
// performing a re-registration.
type ReRegisterClient interface {
	GenerateHostCerts(context.Context, *proto.HostCertsRequest) (*proto.Certs, error)
	Ping(context.Context) (proto.PingResponse, error)
}

// ReRegister renews the certificates based on the client's existing identity.
func ReRegister(ctx context.Context, params ReRegisterParams) (*state.Identity, error) {
	key, err := cryptosuites.GenerateKey(ctx, func(ctx context.Context) (types.SignatureAlgorithmSuite, error) {
		pr, err := params.Client.Ping(ctx)
		if err != nil {
			return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_UNSPECIFIED, trace.Wrap(err, "pinging auth to determine signature algorithm suite")
		}
		return pr.GetSignatureAlgorithmSuite(), nil
	}, cryptosuites.HostIdentity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKeyPEM, err := keys.MarshalPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshPub, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsPub, err := keys.MarshalPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var rotation *types.Rotation
	if !params.Rotation.IsZero() {
		// older auths didn't distinguish between empty and nil rotation
		// structs, so we go out of our way to only send non-nil rotation
		// if it is truly non-empty.
		rotation = &params.Rotation
	}
	certs, err := params.Client.GenerateHostCerts(ctx,
		&proto.HostCertsRequest{
			HostID:                params.ID.HostID(),
			NodeName:              params.ID.NodeName,
			Role:                  params.ID.Role,
			AdditionalPrincipals:  params.AdditionalPrincipals,
			DNSNames:              params.DNSNames,
			RemoteAddr:            params.RemoteAddr,
			PublicTLSKey:          tlsPub,
			PublicSSHKey:          ssh.MarshalAuthorizedKey(sshPub),
			Rotation:              rotation,
			SystemRoles:           params.SystemRoles,
			SystemRoleAssertionID: params.SystemRoleAssertionID,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return state.ReadIdentityFromKeyPair(privateKeyPEM, certs)
}
