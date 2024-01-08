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
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// Keygen is a key generator that precomputes keys to provide quick access to
// public/private key pairs.
type Keygen struct {
	ctx    context.Context
	cancel context.CancelFunc

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
func New(ctx context.Context, opts ...Option) *Keygen {
	ctx, cancel := context.WithCancel(ctx)
	k := &Keygen{
		ctx:    ctx,
		cancel: cancel,
		clock:  clockwork.NewRealClock(),
	}
	for _, opt := range opts {
		opt(k)
	}

	return k
}

// Close stops the precomputation of keys (if enabled) and releases all resources.
func (k *Keygen) Close() {
	k.cancel()
}

// GenerateKeyPair returns fresh priv/pub keypair, takes about 300ms to
// execute.
func (k *Keygen) GenerateKeyPair() ([]byte, []byte, error) {
	return native.GenerateKeyPair()
}

// GenerateHostCert generates a host certificate with the passed in parameters.
// The private key of the CA to sign the certificate must be provided.
func (k *Keygen) GenerateHostCert(c services.HostCertParams) ([]byte, error) {
	if err := c.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	return k.GenerateHostCertWithoutValidation(c)
}

// GenerateHostCertWithoutValidation generates a host certificate with the
// passed in parameters without validating them. For use in tests only.
func (k *Keygen) GenerateHostCertWithoutValidation(c services.HostCertParams) ([]byte, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(c.PublicHostKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Build a valid list of principals from the HostID and NodeName and then
	// add in any additional principals passed in.
	principals := BuildPrincipals(c.HostID, c.NodeName, c.ClusterName, types.SystemRoles{c.Role})
	principals = append(principals, c.Principals...)
	if len(principals) == 0 {
		return nil, trace.BadParameter("no principals provided: %v, %v, %v",
			c.HostID, c.NodeName, c.Principals)
	}
	principals = apiutils.Deduplicate(principals)

	// create certificate
	validBefore := uint64(ssh.CertTimeInfinity)
	if c.TTL != 0 {
		b := k.clock.Now().UTC().Add(c.TTL)
		validBefore = uint64(b.Unix())
	}
	cert := &ssh.Certificate{
		ValidPrincipals: principals,
		Key:             pubKey,
		ValidAfter:      uint64(k.clock.Now().UTC().Add(-1 * time.Minute).Unix()),
		ValidBefore:     validBefore,
		CertType:        ssh.HostCert,
	}
	cert.Permissions.Extensions = make(map[string]string)
	cert.Permissions.Extensions[utils.CertExtensionRole] = c.Role.String()
	cert.Permissions.Extensions[utils.CertExtensionAuthority] = c.ClusterName

	// sign host certificate with private signing key of certificate authority
	if err := cert.SignCert(rand.Reader, c.CASigner); err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debugf("Generated SSH host certificate for role %v with principals: %v.",
		c.Role, principals)
	return ssh.MarshalAuthorizedKey(cert), nil
}

// GenerateUserCert generates a user ssh certificate with the passed in parameters.
// The private key of the CA to sign the certificate must be provided.
func (k *Keygen) GenerateUserCert(c services.UserCertParams) ([]byte, error) {
	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "error validating UserCertParams")
	}
	return k.GenerateUserCertWithoutValidation(c)
}

// GenerateUserCertWithoutValidation generates a user ssh certificate with the
// passed in parameters without validating them.
func (k *Keygen) GenerateUserCertWithoutValidation(c services.UserCertParams) ([]byte, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(c.PublicUserKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	validBefore := uint64(ssh.CertTimeInfinity)
	if c.TTL != 0 {
		b := k.clock.Now().UTC().Add(c.TTL)
		validBefore = uint64(b.Unix())
		log.Debugf("generated user key for %v with expiry on (%v) %v", c.AllowedLogins, validBefore, b)
	}
	cert := &ssh.Certificate{
		// we have to use key id to identify teleport user
		KeyId:           c.Username,
		ValidPrincipals: c.AllowedLogins,
		Key:             pubKey,
		ValidAfter:      uint64(k.clock.Now().UTC().Add(-1 * time.Minute).Unix()),
		ValidBefore:     validBefore,
		CertType:        ssh.UserCert,
	}
	cert.Permissions.Extensions = map[string]string{
		teleport.CertExtensionPermitPTY: "",
	}
	if c.PermitX11Forwarding {
		cert.Permissions.Extensions[teleport.CertExtensionPermitX11Forwarding] = ""
	}
	if c.PermitAgentForwarding {
		cert.Permissions.Extensions[teleport.CertExtensionPermitAgentForwarding] = ""
	}
	if c.PermitPortForwarding {
		cert.Permissions.Extensions[teleport.CertExtensionPermitPortForwarding] = ""
	}
	if c.MFAVerified != "" {
		cert.Permissions.Extensions[teleport.CertExtensionMFAVerified] = c.MFAVerified
	}
	if !c.PreviousIdentityExpires.IsZero() {
		cert.Permissions.Extensions[teleport.CertExtensionPreviousIdentityExpires] = c.PreviousIdentityExpires.Format(time.RFC3339)
	}
	if c.LoginIP != "" {
		cert.Permissions.Extensions[teleport.CertExtensionLoginIP] = c.LoginIP
	}
	if c.Impersonator != "" {
		cert.Permissions.Extensions[teleport.CertExtensionImpersonator] = c.Impersonator
	}
	if c.DisallowReissue {
		cert.Permissions.Extensions[teleport.CertExtensionDisallowReissue] = ""
	}
	if c.Renewable {
		cert.Permissions.Extensions[teleport.CertExtensionRenewable] = ""
	}
	if c.Generation > 0 {
		cert.Permissions.Extensions[teleport.CertExtensionGeneration] = fmt.Sprint(c.Generation)
	}
	if c.BotName != "" {
		cert.Permissions.Extensions[teleport.CertExtensionBotName] = c.BotName
	}
	if c.AllowedResourceIDs != "" {
		cert.Permissions.Extensions[teleport.CertExtensionAllowedResources] = c.AllowedResourceIDs
	}
	if c.ConnectionDiagnosticID != "" {
		cert.Permissions.Extensions[teleport.CertExtensionConnectionDiagnosticID] = c.ConnectionDiagnosticID
	}
	if c.PrivateKeyPolicy != "" {
		cert.Permissions.Extensions[teleport.CertExtensionPrivateKeyPolicy] = string(c.PrivateKeyPolicy)
	}
	if devID := c.DeviceID; devID != "" {
		cert.Permissions.Extensions[teleport.CertExtensionDeviceID] = devID
	}
	if assetTag := c.DeviceAssetTag; assetTag != "" {
		cert.Permissions.Extensions[teleport.CertExtensionDeviceAssetTag] = assetTag
	}
	if credID := c.DeviceCredentialID; credID != "" {
		cert.Permissions.Extensions[teleport.CertExtensionDeviceCredentialID] = credID
	}

	if c.PinnedIP != "" {
		if modules.GetModules().BuildType() != modules.BuildEnterprise {
			return nil, trace.AccessDenied("source IP pinning is only supported in Teleport Enterprise")
		}
		if cert.CriticalOptions == nil {
			cert.CriticalOptions = make(map[string]string)
		}
		//IPv4, all bits matter
		ip := c.PinnedIP + "/32"
		if strings.Contains(c.PinnedIP, ":") {
			//IPv6
			ip = c.PinnedIP + "/128"
		}
		cert.CriticalOptions[teleport.CertCriticalOptionSourceAddress] = ip
	}

	for _, extension := range c.CertificateExtensions {
		// TODO(lxea): update behavior when non ssh, non extensions are supported.
		if extension.Mode != types.CertExtensionMode_EXTENSION ||
			extension.Type != types.CertExtensionType_SSH {
			continue
		}
		cert.Extensions[extension.Name] = extension.Value
	}

	// Add roles, traits, and route to cluster in the certificate extensions if
	// the standard format was requested. Certificate extensions are not included
	// legacy SSH certificates due to a bug in OpenSSH <= OpenSSH 7.1:
	// https://bugzilla.mindrot.org/show_bug.cgi?id=2387
	if c.CertificateFormat == constants.CertificateFormatStandard {
		traits, err := wrappers.MarshalTraits(&c.Traits)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(traits) > 0 {
			cert.Permissions.Extensions[teleport.CertExtensionTeleportTraits] = string(traits)
		}
		if len(c.Roles) != 0 {
			roles, err := services.MarshalCertRoles(c.Roles)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			cert.Permissions.Extensions[teleport.CertExtensionTeleportRoles] = roles
		}
		if c.RouteToCluster != "" {
			cert.Permissions.Extensions[teleport.CertExtensionTeleportRouteToCluster] = c.RouteToCluster
		}
		if !c.ActiveRequests.IsEmpty() {
			requests, err := c.ActiveRequests.Marshal()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			cert.Permissions.Extensions[teleport.CertExtensionTeleportActiveRequests] = string(requests)
		}
	}

	if err := cert.SignCert(rand.Reader, c.CASigner); err != nil {
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
