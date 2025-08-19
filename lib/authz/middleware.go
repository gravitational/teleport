// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package authz

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net"
	"net/http"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var logger = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentAuth)

const (
	// teleportImpersonateUserHeader is a header that specifies teleport user identity
	// that the proxy is impersonating.
	teleportImpersonateUserHeader = "Teleport-Impersonate-User"
	// teleportImpersonateIPHeader is a header that specifies the real user IP address.
	teleportImpersonateIPHeader = "Teleport-Impersonate-IP"
)

// Middleware is authentication middleware that extracts user information
// from the TLS handshake.
type Middleware struct {
	ClusterName string
	// Handler is HTTP handler called after the middleware checks requests
	Handler http.Handler
	// AcceptedUsage restricts authentication
	// to a subset of certificates based on certificate metadata,
	// for example middleware can reject certificates with mismatching usage.
	// If empty, will only accept certificates with non-limited usage,
	// if set, will accept certificates with non-limited usage,
	// and usage exactly matching the specified values.
	AcceptedUsage []string
	// EnableCredentialsForwarding allows the middleware to receive impersonation
	// identity from the client if it presents a valid proxy certificate.
	// This is used by the proxy to forward the identity of the user who
	// connected to the proxy to the next hop.
	EnableCredentialsForwarding bool
}

func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil {
		trace.WriteError(w, trace.AccessDenied("missing authentication"))
		return
	}
	user, err := m.GetUser(r.Context(), *r.TLS)
	if err != nil {
		trace.WriteError(w, err)
		return
	}

	remoteAddr := r.RemoteAddr
	// If the request is coming from a trusted proxy and the proxy is sending a
	// TeleportImpersonateHeader, we will impersonate the user in the header
	// instead of the user in the TLS certificate.
	// This is used by the proxy to impersonate the end user when making requests
	// without re-signing the client certificate.
	impersonateUser := r.Header.Get(teleportImpersonateUserHeader)
	if impersonateUser != "" {
		if !isProxyRole(user) {
			trace.WriteError(w, trace.AccessDenied("Credentials forwarding is only permitted for Proxy"))
			return
		}
		// If the service is not configured to allow credentials forwarding, reject the request.
		if !m.EnableCredentialsForwarding {
			trace.WriteError(w, trace.AccessDenied("Credentials forwarding is not permitted by this service"))
			return
		}

		proxyClusterName := user.GetIdentity().TeleportCluster
		if user, err = m.extractIdentityFromImpersonationHeader(proxyClusterName, impersonateUser); err != nil {
			trace.WriteError(w, err)
			return
		}
		remoteAddr = r.Header.Get(teleportImpersonateIPHeader)
	}

	// If the request is coming from a trusted proxy, we already know the user
	// and we will impersonate him. At this point, we need to remove the
	// TeleportImpersonateHeader from the request, otherwise the proxy will
	// attempt sending the request to upstream servers with the impersonation
	// header from a fake user.
	r.Header.Del(teleportImpersonateUserHeader)
	r.Header.Del(teleportImpersonateIPHeader)

	// determine authenticated user based on the request parameters
	ctx := r.Context()
	ctx = ContextWithUserCertificate(ctx, certFromConnState(r.TLS))
	clientSrcAddr, err := utils.ParseAddr(remoteAddr)
	if err == nil {
		ctx = ContextWithClientSrcAddr(ctx, clientSrcAddr)
	}
	ctx = ContextWithUser(ctx, user)
	r = r.WithContext(ctx)
	// set remote address to the one that was passed in the header
	// this is needed because impersonation reuses the same connection
	// and the remote address is not updated from 0.0.0.0:0
	r.RemoteAddr = remoteAddr
	m.Handler.ServeHTTP(w, r)
}

func certFromConnState(state *tls.ConnectionState) *x509.Certificate {
	if state == nil || len(state.PeerCertificates) != 1 {
		return nil
	}
	return state.PeerCertificates[0]
}

// GetUser returns authenticated user based on request TLS metadata
func (a *Middleware) GetUser(ctx context.Context, connState tls.ConnectionState) (IdentityGetter, error) {
	peers := connState.PeerCertificates
	if len(peers) > 1 {
		// when turning intermediaries on, don't forget to verify
		// https://github.com/kubernetes/kubernetes/pull/34524/files#diff-2b283dde198c92424df5355f39544aa4R59
		return nil, trace.AccessDenied("access denied: intermediaries are not supported")
	}

	// with no client authentication in place, middleware
	// assumes not-privileged Nop role.
	// it theoretically possible to use bearer token auth even
	// for connections without auth, but this is not active use-case
	// therefore it is not allowed to reduce scope
	if len(peers) == 0 {
		return BuiltinRole{
			Role:        types.RoleNop,
			Username:    string(types.RoleNop),
			ClusterName: a.ClusterName,
			Identity:    tlsca.Identity{},
		}, nil
	}
	clientCert := peers[0]

	identity, err := tlsca.FromSubject(clientCert.Subject, clientCert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Since 5.0, teleport TLS certs include the origin teleport cluster in the
	// subject (identity). Before 5.0, origin teleport cluster was inferred
	// from the cert issuer.
	certClusterName := identity.TeleportCluster
	if certClusterName == "" {
		certClusterName, err = tlsca.ClusterName(clientCert.Issuer)
		if err != nil {
			logger.WarnContext(ctx, "Failed to parse client certificate", "error", err)
			return nil, trace.AccessDenied("access denied: invalid client certificate")
		}
		identity.TeleportCluster = certClusterName
	}
	// If there is any restriction on the certificate usage
	// reject the API server request. This is done so some classes
	// of certificates issued for kubernetes usage by proxy, can not be used
	// against auth server. Later on we can extend more
	// advanced cert usage, but for now this is the safest option.
	if len(identity.Usage) != 0 && !slices.Equal(a.AcceptedUsage, identity.Usage) {
		logger.WarnContext(ctx, "Restricted certificate rejected while accessing the auth endpoint",
			"user", identity.Username,
			"cert_usage", identity.Usage,
			"acceptable_usage", a.AcceptedUsage,
		)
		return nil, trace.AccessDenied("access denied: invalid client certificate")
	}

	// this block assumes interactive user from remote cluster
	// based on the remote certificate authority cluster name encoded in
	// x509 organization name. This is a safe check because:
	// 1. Trust and verification is established during TLS handshake
	// by creating a cert pool constructed of trusted certificate authorities
	// 2. Remote CAs are not allowed to have the same cluster name
	// as the local certificate authority
	if certClusterName != a.ClusterName {
		// make sure that this user does not have system role
		// the local auth server can not trust remote servers
		// to issue certificates with system roles (e.g. Admin),
		// to get unrestricted access to the local cluster
		systemRole := findPrimarySystemRole(identity.Groups)
		if systemRole != nil {
			return RemoteBuiltinRole{
				Role:        *systemRole,
				Username:    identity.Username,
				ClusterName: certClusterName,
				Identity:    *identity,
			}, nil
		}

		return RemoteUser{
			ClusterName:      certClusterName,
			Username:         identity.Username,
			Principals:       identity.Principals,
			KubernetesGroups: identity.KubernetesGroups,
			KubernetesUsers:  identity.KubernetesUsers,
			DatabaseNames:    identity.DatabaseNames,
			DatabaseUsers:    identity.DatabaseUsers,
			RemoteRoles:      identity.Groups,
			Identity:         *identity,
		}, nil
	}
	// code below expects user or service from local cluster, to distinguish between
	// interactive users and services (e.g. proxies), the code below
	// checks for presence of system roles issued in certificate identity
	systemRole := findPrimarySystemRole(identity.Groups)
	// in case if the system role is present, assume this is a service
	// agent, e.g. Proxy, connecting to the cluster
	if systemRole != nil {
		return BuiltinRole{
			Role:                  *systemRole,
			AdditionalSystemRoles: extractAdditionalSystemRoles(ctx, identity.SystemRoles),
			Username:              identity.Username,
			ClusterName:           a.ClusterName,
			Identity:              *identity,
		}, nil
	}
	// otherwise assume that is a local role, no need to pass the roles
	// as it will be fetched from the local database
	return LocalUser{
		Username: identity.Username,
		Identity: *identity,
	}, nil
}

// extractIdentityFromImpersonationHeader extracts the identity from the impersonation
// header and returns it. If the impersonation header holds an identity of a
// system role, an error is returned.
func (m *Middleware) extractIdentityFromImpersonationHeader(proxyCluster string, impersonate string) (IdentityGetter, error) {
	// Unmarshal the impersonated user from the header.
	var impersonatedIdentity tlsca.Identity
	if err := json.Unmarshal([]byte(impersonate), &impersonatedIdentity); err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case findPrimarySystemRole(impersonatedIdentity.Groups) != nil:
		// make sure that this user does not have system role
		// since system roles are not allowed to be impersonated.
		return nil, trace.AccessDenied("can not impersonate a system role")
	case proxyCluster != "" && proxyCluster != m.ClusterName && proxyCluster != impersonatedIdentity.TeleportCluster:
		// If a remote proxy is impersonating a user from a different cluster, we
		// must reject the request. This is because the proxy is not allowed to
		// impersonate a user from a different cluster.
		return nil, trace.AccessDenied("can not impersonate users via a different cluster proxy")
	case impersonatedIdentity.TeleportCluster != m.ClusterName:
		// if the impersonated user is from a different cluster, then return a remote user.
		return RemoteUser{
			ClusterName:      impersonatedIdentity.TeleportCluster,
			Username:         impersonatedIdentity.Username,
			Principals:       impersonatedIdentity.Principals,
			KubernetesGroups: impersonatedIdentity.KubernetesGroups,
			KubernetesUsers:  impersonatedIdentity.KubernetesUsers,
			DatabaseNames:    impersonatedIdentity.DatabaseNames,
			DatabaseUsers:    impersonatedIdentity.DatabaseUsers,
			RemoteRoles:      impersonatedIdentity.Groups,
			Identity:         impersonatedIdentity,
		}, nil
	default:
		// otherwise assume that is a local role, no need to pass the roles
		// as it will be fetched from the local database
		return LocalUser{
			Username: impersonatedIdentity.Username,
			Identity: impersonatedIdentity,
		}, nil
	}
}

func extractAdditionalSystemRoles(ctx context.Context, roles []string) types.SystemRoles {
	var systemRoles types.SystemRoles
	for _, role := range roles {
		systemRole := types.SystemRole(role)
		err := systemRole.Check()
		if err != nil {
			// ignore unknown system roles rather than rejecting them, since new unknown system
			// roles may be present on certs if we rolled back from a newer version.
			logger.WarnContext(ctx, "Ignoring unknown system role", "unknown_role", role)
			continue
		}
		systemRoles = append(systemRoles, systemRole)
	}
	return systemRoles
}

func findPrimarySystemRole(roles []string) *types.SystemRole {
	for _, role := range roles {
		systemRole := types.SystemRole(role)
		err := systemRole.Check()
		if err == nil {
			return &systemRole
		}
	}
	return nil
}

// isProxyRole returns true if the certificate role is a proxy role.
func isProxyRole(identity IdentityGetter) bool {
	switch id := identity.(type) {
	case RemoteBuiltinRole:
		return id.Role == types.RoleProxy
	case BuiltinRole:
		return id.Role == types.RoleProxy
	default:
		return false
	}
}

// WrapContextWithUser enriches the provided context with the identity information
// extracted from the provided TLS connection.
func (m *Middleware) WrapContextWithUser(ctx context.Context, conn utils.TLSConn) (context.Context, error) {
	// Perform the handshake if it hasn't been already. Before the handshake we
	// won't have client certs available.
	if !conn.ConnectionState().HandshakeComplete {
		if err := conn.HandshakeContext(ctx); err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	}

	return m.WrapContextWithUserFromTLSConnState(ctx, conn.ConnectionState(), conn.RemoteAddr())
}

// WrapContextWithUserFromTLSConnState enriches the provided context with the identity information
// extracted from the provided TLS connection state.
func (m *Middleware) WrapContextWithUserFromTLSConnState(ctx context.Context, tlsState tls.ConnectionState, remoteAddr net.Addr) (context.Context, error) {
	user, err := m.GetUser(ctx, tlsState)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx = ContextWithUserCertificate(ctx, certFromConnState(&tlsState))
	ctx = ContextWithClientSrcAddr(ctx, remoteAddr)
	ctx = ContextWithUser(ctx, user)
	return ctx, nil
}
