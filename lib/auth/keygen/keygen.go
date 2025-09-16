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

package keygen

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/sshca"
)

// Keygen is a key generator that precomputes keys to provide quick access to
// public/private key pairs.
type Keygen struct {
	// clock is used to control time.
	clock clockwork.Clock
}

// Option is a functional optional argument for key generator
type Option func(k *Keygen)

// SetClock sets the clock to use for key generation.
func SetClock(clock clockwork.Clock) Option {
	return func(k *Keygen) {
		k.clock = clock
	}
}

// New returns a new key generator.
func New(_ context.Context, opts ...Option) *Keygen {
	k := &Keygen{
		clock: clockwork.NewRealClock(),
	}
	for _, opt := range opts {
		opt(k)
	}

	return k
}

// GenerateKeyPair returns fresh priv/pub keypair, takes about 300ms to execute.
func (k *Keygen) GenerateKeyPair() ([]byte, []byte, error) {
	return native.GenerateKeyPair()
}

// GenerateHostCert generates a host certificate with the passed in parameters.
// The private key of the CA to sign the certificate must be provided.
func (k *Keygen) GenerateHostCert(req sshca.HostCertificateRequest) ([]byte, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	return k.GenerateHostCertWithoutValidation(req)
}

// GenerateHostCertWithoutValidation generates a host certificate with the
// passed in parameters without validating them. For use in tests only.
func (k *Keygen) GenerateHostCertWithoutValidation(req sshca.HostCertificateRequest) ([]byte, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(req.PublicHostKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create shallow copy of identity since we want to make some local changes
	ident := req.Identity

	ident.CertType = ssh.HostCert

	// Build a valid list of principals from the HostID and NodeName and then
	// add in any additional principals passed in.
	principals := BuildPrincipals(req.HostID, req.NodeName, ident.ClusterName, types.SystemRoles{ident.SystemRole})
	principals = append(principals, ident.Principals...)
	if len(principals) == 0 {
		return nil, trace.BadParameter("cannot generate host certificate without principals")
	}
	principals = apiutils.Deduplicate(principals)
	ident.Principals = principals

	// calculate ValidBefore based on the outer request TTL
	ident.ValidBefore = uint64(ssh.CertTimeInfinity)
	if req.TTL != 0 {
		b := k.clock.Now().UTC().Add(req.TTL)
		ident.ValidBefore = uint64(b.Unix())
	}

	ident.ValidAfter = uint64(k.clock.Now().UTC().Add(-1 * time.Minute).Unix())

	// encode the identity into a certificate
	cert, err := ident.Encode("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// set the public key of the certificate
	cert.Key = pubKey

	// sign host certificate with private signing key of certificate authority
	if err := cert.SignCert(rand.Reader, req.CASigner); err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debugf("Generated SSH host certificate for role %v with principals: %v.",
		ident.SystemRole, ident.Principals)

	return ssh.MarshalAuthorizedKey(cert), nil
}

// GenerateUserCert generates a user ssh certificate with the passed in parameters.
// The private key of the CA to sign the certificate must be provided.
func (k *Keygen) GenerateUserCert(req sshca.UserCertificateRequest) ([]byte, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "error validating user certificate request")
	}
	return k.GenerateUserCertWithoutValidation(req)
}

// GenerateUserCertWithoutValidation generates a user ssh certificate with the
// passed in parameters without validating them.
func (k *Keygen) GenerateUserCertWithoutValidation(req sshca.UserCertificateRequest) ([]byte, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(req.PublicUserKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create shallow copy of identity since we want to make some local changes
	ident := req.Identity

	ident.CertType = ssh.UserCert

	// calculate ValidBefore based on the outer request TTL
	ident.ValidBefore = uint64(ssh.CertTimeInfinity)
	if req.TTL != 0 {
		b := k.clock.Now().UTC().Add(req.TTL)
		ident.ValidBefore = uint64(b.Unix())
		log.Debugf("generated user key for %v with expiry on (%v) %v", ident.Principals, ident.ValidBefore, b)
	}

	// set ValidAfter to be 1 minute in the past
	ident.ValidAfter = uint64(k.clock.Now().UTC().Add(-1 * time.Minute).Unix())

	// if the provided identity is attempting to perform IP pinning, make sure modules are enforced
	if ident.PinnedIP != "" {
		if modules.GetModules().BuildType() != modules.BuildEnterprise {
			return nil, trace.AccessDenied("source IP pinning is only supported in Teleport Enterprise")
		}
	}

	// encode the identity into a certificate
	cert, err := ident.Encode(req.CertificateFormat)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// set the public key of the certificate
	cert.Key = pubKey

	if err := cert.SignCert(rand.Reader, req.CASigner); err != nil {
		return nil, trace.Wrap(err)
	}

	return ssh.MarshalAuthorizedKey(cert), nil
}

// BuildPrincipals takes a hostID, nodeName, clusterName, and role and builds a list of
// principals to insert into a certificate. This function is backward compatible with
// older clients which means:
//   - If RoleAdmin is in the list of roles, only a single principal is returned: hostID
//   - If nodename is empty, it is not included in the list of principals.
func BuildPrincipals(hostID string, nodeName string, clusterName string, roles types.SystemRoles) []string {
	// TODO(russjones): This should probably be clusterName, but we need to
	// verify changing this won't break older clients.
	if roles.Include(types.RoleAdmin) {
		return []string{hostID}
	}

	// if no hostID was passed it, the user might be specifying an exact list of principals
	if hostID == "" {
		return []string{}
	}

	// always include the hostID, this is what teleport uses internally to find nodes
	principals := []string{
		fmt.Sprintf("%v.%v", hostID, clusterName),
		hostID,
	}

	// nodeName is the DNS name, this is for OpenSSH interoperability
	if nodeName != "" {
		principals = append(principals, fmt.Sprintf("%s.%s", nodeName, clusterName))
		principals = append(principals, nodeName)
	}

	// Add localhost and loopback addresses to allow connecting to proxy/host
	// on the local machine. This should only matter for quickstart and local
	// development.
	principals = append(principals,
		string(teleport.PrincipalLocalhost),
		string(teleport.PrincipalLoopbackV4),
		string(teleport.PrincipalLoopbackV6),
	)

	// deduplicate (in-case hostID and nodeName are the same) and return
	return apiutils.Deduplicate(principals)
}
