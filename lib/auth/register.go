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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/defaults"
)

// LocalRegister is used to generate host keys when a node or proxy is running
// within the same process as the Auth Server and as such, does not need to
// use provisioning tokens.
func LocalRegister(id state.IdentityID, authServer *Server, additionalPrincipals, dnsNames []string, remoteAddr string, systemRoles []types.SystemRole) (*state.Identity, error) {
	priv, pub, err := native.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsPub, err := PrivateKeyToPublicKeyTLS(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If local registration is happening and no remote address was passed in
	// (which means no advertise IP was set), use localhost.
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
			PublicSSHKey:         pub,
			PublicTLSKey:         tlsPub,
			SystemRoles:          systemRoles,
		})
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
	// PrivateKey is a PEM encoded private key (not passed to auth servers)
	PrivateKey []byte
	// PublicTLSKey is a server's public key to sign
	PublicTLSKey []byte
	// PublicSSHKey is a server's public SSH key to sign
	PublicSSHKey []byte
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
}

// ReRegister renews the certificates and private keys based on the client's existing identity.
func ReRegister(ctx context.Context, params ReRegisterParams) (*state.Identity, error) {
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
			PublicTLSKey:          params.PublicTLSKey,
			PublicSSHKey:          params.PublicSSHKey,
			Rotation:              rotation,
			SystemRoles:           params.SystemRoles,
			SystemRoleAssertionID: params.SystemRoleAssertionID,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return state.ReadIdentityFromKeyPair(params.PrivateKey, certs)
}
