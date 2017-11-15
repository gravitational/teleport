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

package srv

import (
	"fmt"
	"os"
	"os/user"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// AuthHandlers are common authorization and authentication related handlers
// used by the regular and forwarding server.
type AuthHandlers struct {
	*log.Entry

	// Server is the services.Server in the backend.
	Server services.Server

	// Component is the type of SSH server (node, proxy, or recording proxy).
	Component string

	// AuditLog is the service used to access Audit Log.
	AuditLog events.IAuditLog

	// AccessPoint is used to access the Auth Server.
	AccessPoint auth.AccessPoint
}

// CheckAgentForward checks the certificate was signed by a valid certificate
// authority, fetches the appropriate roles, and checks if agent forwarding
// is allowed.
func (h *AuthHandlers) CheckAgentForward(ctx *ServerContext) error {
	cert, err := ctx.GetCertificate()
	if err != nil {
		return trace.Wrap(err)
	}

	ca, err := h.authorityForCert(cert.SignatureKey)
	if err != nil {
		return trace.Wrap(err)
	}

	roles, err := h.fetchRoleSet(cert, ca, ctx.TeleportUser, ctx.ClusterName)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := roles.CheckAgentForward(ctx.Login); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// KeyAuth implements SSH client authentication using public keys and is called
// by the server every time the client connects
func (h *AuthHandlers) KeyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	fingerprint := fmt.Sprintf("%v %v", key.Type(), sshutils.Fingerprint(key))

	// as soon as key auth starts, we know something about the connection, so
	// update *log.Entry.
	h.Entry = log.WithFields(log.Fields{
		trace.Component: h.Component,
		trace.ComponentFields: log.Fields{
			"local":       conn.LocalAddr(),
			"remote":      conn.RemoteAddr(),
			"user":        conn.User(),
			"fingerprint": fingerprint,
		},
	})

	cid := fmt.Sprintf("conn(%v->%v, user=%v)", conn.RemoteAddr(), conn.LocalAddr(), conn.User())
	h.Debugf("%v auth attempt", cid)

	certChecker := ssh.CertChecker{IsAuthority: h.isAuthority}

	cert, ok := key.(*ssh.Certificate)
	h.Debugf("%v auth attempt with key %v, %#v", cid, fingerprint, cert)
	if !ok {
		h.Debugf("auth attempt, unsupported key type")
		return nil, trace.BadParameter("unsupported key type: %v", fingerprint)
	}
	if len(cert.ValidPrincipals) == 0 {
		h.Debugf("need a valid principal for key")
		return nil, trace.BadParameter("need a valid principal for key %v", fingerprint)
	}

	if len(cert.KeyId) == 0 {
		h.Debugf("need a valid key ID for key")
		return nil, trace.BadParameter("need a valid key for key %v", fingerprint)
	}
	teleportUser := cert.KeyId

	// only failed attempts are logged right now
	recordFailedLogin := func(err error) {
		fields := events.EventFields{
			events.EventUser:          teleportUser,
			events.AuthAttemptSuccess: false,
			events.AuthAttemptErr:     err.Error(),
		}
		h.Warningf("failed login attempt %#v", fields)
		h.AuditLog.EmitAuditEvent(events.AuthAttemptEvent, fields)
	}
	permissions, err := certChecker.Authenticate(conn, key)
	if err != nil {
		recordFailedLogin(err)
		return nil, trace.Wrap(err)
	}
	if err := certChecker.CheckCert(conn.User(), cert); err != nil {
		recordFailedLogin(err)
		return nil, trace.Wrap(err)
	}
	h.Debugf("successfully authenticated")

	// see if the host user is valid (no need to do this in proxy mode)
	if !h.isProxy() {
		_, err = user.Lookup(conn.User())
		if err != nil {
			host, _ := os.Hostname()
			h.Warningf("host '%s' does not have OS user '%s'", host, conn.User())
			h.Errorf("no such user")
			return nil, trace.AccessDenied("no such user: '%s'", conn.User())
		}
	}

	clusterName, err := h.AccessPoint.GetDomainName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// this is the only way we know of to pass valid additional data about the
	// connection to the handlers
	permissions.Extensions[utils.CertTeleportUser] = teleportUser
	permissions.Extensions[utils.CertTeleportClusterName] = clusterName
	permissions.Extensions[utils.CertTeleportUserCertificate] = string(ssh.MarshalAuthorizedKey(cert))

	if h.isProxy() {
		return permissions, nil
	}

	// if we are trying to connect to a node, make sure RBAC rules allow it
	err = h.checkPermissionToLogin(cert, clusterName, teleportUser, conn.User())
	if err != nil {
		h.Errorf("Permission denied: %v", err)
		recordFailedLogin(err)
		return nil, trace.Wrap(err)
	}

	return permissions, nil
}

// checkPermissionToLogin checks the given certificate (supplied by a connected
// client) to see if this certificate can be allowed to login as user:login
// pair to requested server.
func (h *AuthHandlers) checkPermissionToLogin(cert *ssh.Certificate, clusterName string, teleportUser, osUser string) error {
	h.Debugf("CheckPermsissionToLogin(%v, %v)", teleportUser, osUser)

	// get the ca that signd the users certificate
	ca, err := h.authorityForCert(cert.SignatureKey)
	if err != nil {
		return trace.Wrap(err)
	}

	// get roles assigned to this user
	roles, err := h.fetchRoleSet(cert, ca, teleportUser, clusterName)
	if err != nil {
		return trace.Wrap(err)
	}

	// check if roles allow access to server
	if err := roles.CheckAccessToServer(osUser, h.Server); err != nil {
		return trace.AccessDenied("user %s@%s is not authorized to login as %v@%s: %v",
			teleportUser, ca.GetClusterName(), osUser, clusterName, err)
	}

	return nil
}

// fetchRoleSet fetches the services.RoleSet assigned to a Teleport user.
func (h *AuthHandlers) fetchRoleSet(cert *ssh.Certificate, ca services.CertAuthority, teleportUser string, clusterName string) (services.RoleSet, error) {
	// for local users, go and check their individual permissions
	var roles services.RoleSet
	if clusterName == ca.GetClusterName() {
		users, err := h.AccessPoint.GetUsers()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, u := range users {
			if u.GetName() == teleportUser {
				// pass along the traits so we get the substituted roles for this user
				roles, err = services.FetchRoles(u.GetRoles(), h.AccessPoint, u.GetTraits())
				if err != nil {
					return nil, trace.Wrap(err)
				}
			}
		}
	} else {
		certRoles, err := extractRolesFromCert(cert)
		if err != nil {
			return nil, trace.AccessDenied("failed to parse certificate roles")
		}
		roleNames, err := ca.CombinedMapping().Map(certRoles)
		if err != nil {
			return nil, trace.AccessDenied("failed to map roles")
		}
		// pass the principals on the certificate along as the login traits
		// to the remote cluster.
		traits := map[string][]string{
			teleport.TraitLogins: cert.ValidPrincipals,
		}
		roles, err = services.FetchRoles(roleNames, h.AccessPoint, traits)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return roles, nil
}

// isAuthority is called during checking the client key, to see if the signing
// key is the real CA authority key.
func (h *AuthHandlers) isAuthority(cert ssh.PublicKey) bool {
	if _, err := h.authorityForCert(cert); err != nil {
		return false
	}
	return true
}

// authorityForCert checks if the certificate was signed by a Teleport
// Certificate Authority and returns it.
func (h *AuthHandlers) authorityForCert(cert ssh.PublicKey) (services.CertAuthority, error) {
	// get all user certificate authorities
	cas, err := h.AccessPoint.GetCertAuthorities(services.UserCA, false)
	if err != nil {
		h.Warningf("%v", trace.DebugReport(err))
		return nil, trace.Wrap(err)
	}

	// find the one that signed our certificate
	var ca services.CertAuthority
	for i := range cas {
		checkers, err := cas[i].Checkers()
		if err != nil {
			h.Warningf("%v", err)
			return nil, trace.Wrap(err)
		}
		for _, checker := range checkers {
			if sshutils.KeysEqual(cert, checker) {
				ca = cas[i]
				break
			}
		}
	}

	// the certificate was signed by unknown authority
	if ca == nil {
		return nil, trace.AccessDenied("the certificate signed by untrusted CA")
	}

	return ca, nil
}

// isProxy returns true if it's a regular SSH proxy.
func (h *AuthHandlers) isProxy() bool {
	if h.Component == teleport.ComponentProxy {
		return true
	}
	return false
}

// extractRolesFromCert extracts roles from certificate metadata extensions.
func extractRolesFromCert(cert *ssh.Certificate) ([]string, error) {
	data, ok := cert.Extensions[teleport.CertExtensionTeleportRoles]
	if !ok {
		// it's ok to not have any roles in the metadata
		return nil, nil
	}
	return services.UnmarshalCertRoles(data)
}
