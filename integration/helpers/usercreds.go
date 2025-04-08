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

package helpers

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
)

// UserCreds holds user client credentials
type UserCreds struct {
	// Key is user client key and certificate
	Key client.Key
	// HostCA is a trusted host certificate authority
	HostCA types.CertAuthority
}

// SetupUserCreds sets up user credentials for client
func SetupUserCreds(tc *client.TeleportClient, proxyHost string, creds UserCreds) error {
	err := tc.AddKey(&creds.Key)
	if err != nil {
		return trace.Wrap(err)
	}
	err = tc.AddTrustedCA(context.Background(), creds.HostCA)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SetupUser sets up user in the cluster
func SetupUser(process *service.TeleportProcess, username string, roles []types.Role) error {
	ctx := context.TODO()
	auth := process.GetAuthServer()
	teleUser, err := types.NewUser(username)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(roles) == 0 {
		role := services.RoleForUser(teleUser)
		role.SetLogins(types.Allow, []string{username})

		// allow tests to forward agent, still needs to be passed in client
		roleOptions := role.GetOptions()
		roleOptions.ForwardAgent = types.NewBool(true)
		role.SetOptions(roleOptions)

		role, err = auth.UpsertRole(ctx, role)
		if err != nil {
			return trace.Wrap(err)
		}
		teleUser.AddRole(role.GetMetadata().Name)
	} else {
		for _, role := range roles {
			role, err := auth.UpsertRole(ctx, role)
			if err != nil {
				return trace.Wrap(err)
			}
			teleUser.AddRole(role.GetName())
		}
	}
	_, err = auth.UpsertUser(ctx, teleUser)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UserCredsRequest is a request to generate user creds
type UserCredsRequest struct {
	// Process is a teleport process
	Process *service.TeleportProcess
	// Username is a user to generate certs for
	Username string
	// RouteToCluster is an optional cluster to route creds to
	RouteToCluster string
	// SourceIP is an optional source IP to use in SSH certs
	SourceIP string
	// TTL is an optional TTL for the certs. Defaults to one hour.
	TTL time.Duration
}

// GenerateUserCreds generates key to be used by client
func GenerateUserCreds(req UserCredsRequest) (*UserCreds, error) {
	ttl := req.TTL
	if ttl == 0 {
		ttl = time.Hour
	}

	priv, err := testauthority.New().GeneratePrivateKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a := req.Process.GetAuthServer()
	sshPub := ssh.MarshalAuthorizedKey(priv.SSHPublicKey())
	sshCert, x509Cert, err := a.GenerateUserTestCerts(auth.GenerateUserTestCertsRequest{
		Key:            sshPub,
		Username:       req.Username,
		TTL:            ttl,
		Compatibility:  constants.CertificateFormatStandard,
		RouteToCluster: req.RouteToCluster,
		PinnedIP:       req.SourceIP,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := a.GetCertAuthority(context.Background(), types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &UserCreds{
		HostCA: ca,
		Key: client.Key{
			PrivateKey: priv,
			Cert:       sshCert,
			TLSCert:    x509Cert,
		},
	}, nil
}
