/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/identity"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
)

// LocalRegister is used to generate host keys when a node or proxy is running
// within the same process as the Auth Server and as such, does not need to
// use provisioning tokens.
func LocalRegister(id identity.IdentityID, authServer *Server, additionalPrincipals, dnsNames []string, remoteAddr string, systemRoles []types.SystemRole) (*Identity, error) {
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

	ident, err := identity.ReadIdentityFromKeyPair(priv, certs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ident, nil
}
