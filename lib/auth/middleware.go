/*
Copyright 2017 Gravitational, Inc.

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
	"crypto/tls"
	"crypto/x509"
	"math"
	"net"
	"net/http"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/http2"
)

// TLSServerConfig is a configuration for TLS server
type TLSServerConfig struct {
	// TLS is a base TLS configuration
	TLS *tls.Config
	// API is API server configuration
	APIConfig
	// LimiterConfig is limiter config
	LimiterConfig limiter.LimiterConfig
	// AccessPoint is a caching access point
	AccessPoint AccessCache
	// Component is used for debugging purposes
	Component string
	// AcceptedUsage restricts authentication
	// to a subset of certificates based on the metadata
	AcceptedUsage []string
}

// CheckAndSetDefaults checks and sets default values
func (c *TLSServerConfig) CheckAndSetDefaults() error {
	if c.TLS == nil {
		return trace.BadParameter("missing parameter TLS")
	}
	c.TLS.ClientAuth = tls.VerifyClientCertIfGiven
	if c.TLS.ClientCAs == nil {
		return trace.BadParameter("missing parameter TLS.ClientCAs")
	}
	if c.TLS.RootCAs == nil {
		return trace.BadParameter("missing parameter TLS.RootCAs")
	}
	if len(c.TLS.Certificates) == 0 {
		return trace.BadParameter("missing parameter TLS.Certificates")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing parameter AccessPoint")
	}
	if c.Component == "" {
		c.Component = teleport.ComponentAuth
	}
	return nil
}

// TLSServer is TLS auth server
type TLSServer struct {
	*http.Server
	// TLSServerConfig is TLS server configuration used for auth server
	TLSServerConfig
	// Entry is TLS server logging entry
	*logrus.Entry
}

// NewTLSServer returns new unstarted TLS server
func NewTLSServer(cfg TLSServerConfig) (*TLSServer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	// limiter limits requests by frequency and amount of simultaneous
	// connections per client
	limiter, err := limiter.NewLimiter(cfg.LimiterConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// authMiddleware authenticates request assuming TLS client authentication
	// adds authentication information to the context
	// and passes it to the API server
	authMiddleware := &AuthMiddleware{
		AccessPoint:   cfg.AccessPoint,
		AcceptedUsage: cfg.AcceptedUsage,
	}
	authMiddleware.Wrap(NewGRPCServer(cfg.APIConfig))
	// Wrap sets the next middleware in chain to the authMiddleware
	limiter.WrapHandle(authMiddleware)
	// force client auth if given
	cfg.TLS.ClientAuth = tls.VerifyClientCertIfGiven
	cfg.TLS.NextProtos = []string{http2.NextProtoTLS}

	server := &TLSServer{
		TLSServerConfig: cfg,
		Server: &http.Server{
			Handler:           limiter,
			ReadHeaderTimeout: defaults.DefaultDialTimeout,
		},
		Entry: logrus.WithFields(logrus.Fields{
			trace.Component: cfg.Component,
		}),
	}
	server.TLS.GetConfigForClient = server.GetConfigForClient
	return server, nil
}

// Serve takes TCP listener, upgrades to TLS using config and starts serving
func (t *TLSServer) Serve(listener net.Listener) error {
	return t.Server.Serve(tls.NewListener(listener, t.TLS))
}

// GetConfigForClient is getting called on every connection
// and server's GetConfigForClient reloads the list of trusted
// local and remote certificate authorities
func (t *TLSServer) GetConfigForClient(info *tls.ClientHelloInfo) (*tls.Config, error) {
	var clusterName string
	var err error
	switch info.ServerName {
	case "":
		// Client does not use SNI, will validate against all known CAs.
	case teleport.APIDomain:
		// REMOVE IN 4.4: all 4.3+ clients must specify the correct cluster name.
		//
		// Instead, this case should either default to current cluster CAs or
		// return an error.
		t.Debugf("Client %q sent %q in SNI, which causes this auth server to send all known CAs in TLS handshake. If this client is version 4.2 or older, this is expected; if this client is version 4.3 or above, please let us know at https://github.com/gravitational/teleport/issues/new", info.Conn.RemoteAddr(), info.ServerName)
	default:
		clusterName, err = DecodeClusterName(info.ServerName)
		if err != nil {
			if !trace.IsNotFound(err) {
				t.Warningf("Client sent unsupported cluster name %q, what resulted in error %v.", info.ServerName, err)
				return nil, trace.AccessDenied("access is denied")
			}
		}
	}

	// update client certificate pool based on currently trusted TLS
	// certificate authorities.
	// TODO(klizhentas) drop connections of the TLS cert authorities
	// that are not trusted
	pool, err := ClientCertPool(t.AccessPoint, clusterName)
	if err != nil {
		var ourClusterName string
		if clusterName, err := t.AccessPoint.GetClusterName(); err == nil {
			ourClusterName = clusterName.GetClusterName()
		}
		t.Errorf("Failed to retrieve client pool. Client cluster %v, target cluster %v, error:  %v.", clusterName, ourClusterName, trace.DebugReport(err))
		// this falls back to the default config
		return nil, nil
	}

	// Per https://tools.ietf.org/html/rfc5246#section-7.4.4 the total size of
	// the known CA subjects sent to the client can't exceed 2^16-1 (due to
	// 2-byte length encoding). The crypto/tls stack will panic if this
	// happens. To make the error less cryptic, catch this condition and return
	// a better error.
	//
	// This may happen with a very large (>500) number of trusted clusters, if
	// the client doesn't send the correct ServerName in its ClientHelloInfo
	// (see the switch at the top of this func).
	var totalSubjectsLen int64
	for _, s := range pool.Subjects() {
		// Each subject in the list gets a separate 2-byte length prefix.
		totalSubjectsLen += 2
		totalSubjectsLen += int64(len(s))
	}
	if totalSubjectsLen >= int64(math.MaxUint16) {
		return nil, trace.BadParameter("number of CAs in client cert pool is too large (%d) and cannot be encoded in a TLS handshake; this is due to a large number of trusted clusters; try updating tsh to the latest version; if that doesn't help, remove some trusted clusters", len(pool.Subjects()))
	}

	tlsCopy := t.TLS.Clone()
	tlsCopy.ClientCAs = pool
	for _, cert := range tlsCopy.Certificates {
		t.Debugf("Server certificate %v.", TLSCertInfo(&cert))
	}
	return tlsCopy, nil
}

// AuthMiddleware is authentication middleware checking every request
type AuthMiddleware struct {
	// AccessPoint is a caching access point for auth server
	AccessPoint AccessCache
	// Handler is HTTP handler called after the middleware checks requests
	Handler http.Handler
	// AcceptedUsage restricts authentication
	// to a subset of certificates based on certificate metadata,
	// for example middleware can reject certificates with mismatching usage.
	// If empty, will only accept certificates with non-limited usage,
	// if set, will accept certificates with non-limited usage,
	// and usage exactly matching the specified values.
	AcceptedUsage []string
}

// Wrap sets next handler in chain
func (a *AuthMiddleware) Wrap(h http.Handler) {
	a.Handler = h
}

// GetUser returns authenticated user based on request metadata set by HTTP server
func (a *AuthMiddleware) GetUser(r *http.Request) (IdentityGetter, error) {
	peers := r.TLS.PeerCertificates
	if len(peers) > 1 {
		// when turning intermediaries on, don't forget to verify
		// https://github.com/kubernetes/kubernetes/pull/34524/files#diff-2b283dde198c92424df5355f39544aa4R59
		return nil, trace.AccessDenied("access denied: intermediaries are not supported")
	}
	localClusterName, err := a.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// with no client authentication in place, middleware
	// assumes not-privileged Nop role.
	// it theoretically possible to use bearer token auth even
	// for connections without auth, but this is not active use-case
	// therefore it is not allowed to reduce scope
	if len(peers) == 0 {
		return BuiltinRole{
			GetClusterConfig: a.AccessPoint.GetClusterConfig,
			Role:             teleport.RoleNop,
			Username:         string(teleport.RoleNop),
			ClusterName:      localClusterName.GetClusterName(),
			Identity:         tlsca.Identity{},
		}, nil
	}
	clientCert := peers[0]
	certClusterName, err := tlsca.ClusterName(clientCert.Issuer)
	if err != nil {
		log.Warnf("Failed to parse client certificate %v.", err)
		return nil, trace.AccessDenied("access denied: invalid client certificate")
	}

	identity, err := tlsca.FromSubject(clientCert.Subject, clientCert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// If there is any restriction on the certificate usage
	// reject the API server request. This is done so some classes
	// of certificates issued for kubernetes usage by proxy, can not be used
	// against auth server. Later on we can extend more
	// advanced cert usage, but for now this is the safest option.
	if len(identity.Usage) != 0 && !utils.StringSlicesEqual(a.AcceptedUsage, identity.Usage) {
		log.Warningf("Restricted certificate of user %q with usage %v rejected while accessing the auth endpoint with acceptable usage %v.",
			identity.Username, identity.Usage, a.AcceptedUsage)
		return nil, trace.AccessDenied("access denied: invalid client certificate")
	}

	// this block assumes interactive user from remote cluster
	// based on the remote certificate authority cluster name encoded in
	// x509 organization name. This is a safe check because:
	// 1. Trust and verification is established during TLS handshake
	// by creating a cert pool constructed of trusted certificate authorities
	// 2. Remote CAs are not allowed to have the same cluster name
	// as the local certificate authority
	if certClusterName != localClusterName.GetClusterName() {
		// make sure that this user does not have system role
		// the local auth server can not truste remote servers
		// to issue certificates with system roles (e.g. Admin),
		// to get unrestricted access to the local cluster
		systemRole := findSystemRole(identity.Groups)
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
			RemoteRoles:      identity.Groups,
			Identity:         *identity,
		}, nil
	}
	// code below expects user or service from local cluster, to distinguish between
	// interactive users and services (e.g. proxies), the code below
	// checks for presence of system roles issued in certificate identity
	systemRole := findSystemRole(identity.Groups)
	// in case if the system role is present, assume this is a service
	// agent, e.g. Proxy, connecting to the cluster
	if systemRole != nil {
		return BuiltinRole{
			GetClusterConfig: a.AccessPoint.GetClusterConfig,
			Role:             *systemRole,
			Username:         identity.Username,
			ClusterName:      localClusterName.GetClusterName(),
			Identity:         *identity,
		}, nil
	}
	// otherwise assume that is a local role, no need to pass the roles
	// as it will be fetched from the local database
	return LocalUser{
		Username: identity.Username,
		Identity: *identity,
	}, nil
}

func findSystemRole(roles []string) *teleport.Role {
	for _, role := range roles {
		systemRole := teleport.Role(role)
		err := systemRole.Check()
		if err == nil {
			return &systemRole
		}
	}
	return nil
}

// ServeHTTP serves HTTP requests
func (a *AuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	baseContext := r.Context()
	if baseContext == nil {
		baseContext = context.TODO()
	}
	user, err := a.GetUser(r)
	if err != nil {
		trace.WriteError(w, err)
		return
	}

	// determine authenticated user based on the request parameters
	requestWithContext := r.WithContext(context.WithValue(baseContext, ContextUser, user))
	a.Handler.ServeHTTP(w, requestWithContext)
}

// ClientCertPool returns trusted x509 cerificate authority pool
func ClientCertPool(client AccessCache, clusterName string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	var authorities []services.CertAuthority
	if clusterName == "" {
		hostCAs, err := client.GetCertAuthorities(services.HostCA, false, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		userCAs, err := client.GetCertAuthorities(services.UserCA, false, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		authorities = append(authorities, hostCAs...)
		authorities = append(authorities, userCAs...)
	} else {
		hostCA, err := client.GetCertAuthority(
			services.CertAuthID{Type: services.HostCA, DomainName: clusterName},
			false, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		userCA, err := client.GetCertAuthority(
			services.CertAuthID{Type: services.UserCA, DomainName: clusterName},
			false, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		authorities = append(authorities, hostCA)
		authorities = append(authorities, userCA)
	}

	for _, auth := range authorities {
		for _, keyPair := range auth.GetTLSKeyPairs() {
			cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			log.Debugf("ClientCertPool -> %v", CertInfo(cert))
			pool.AddCert(cert)
		}
	}
	return pool, nil
}
