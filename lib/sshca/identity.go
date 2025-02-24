/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

// Package sshca specifies interfaces for SSH certificate authorities
package sshca

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/utils"
)

// Identity is a user identity. All identity fields map directly to an ssh certificate field.
type Identity struct {

	// --- common identity fields ---

	// ValidAfter is the unix timestamp that marks the start time for when the certificate should
	// be considered valid.
	ValidAfter uint64
	// ValidBefore is the unix timestamp that marks the end time for when the certificate should
	// be considered valid.
	ValidBefore uint64
	// CertType indicates what type of cert this is (user or host).
	CertType uint32
	// Principals is the list of SSH principals associated with the certificate (this means the
	// list of allowed unix logins in the case of user certs).
	Principals []string

	// --- host identity fields ---

	// ClusterName is the name of the cluster within which a node lives
	ClusterName string
	// SystemRole identifies the system role of a Teleport instance
	SystemRole types.SystemRole

	// -- user identity fields ---

	// Username is teleport username
	Username string
	// Impersonator is set when a user requests certificate for another user
	Impersonator string
	// PermitX11Forwarding permits X11 forwarding for this cert
	PermitX11Forwarding bool
	// PermitAgentForwarding permits agent forwarding for this cert
	PermitAgentForwarding bool
	// PermitPortForwarding permits port forwarding.
	PermitPortForwarding bool
	// Roles is a list of roles assigned to this user
	Roles []string
	// RouteToCluster specifies the target cluster
	// if present in the certificate, will be used
	// to route the requests to
	RouteToCluster string
	// Traits hold claim data used to populate a role at runtime.
	Traits wrappers.Traits
	// ActiveRequests tracks privilege escalation requests applied during
	// certificate construction.
	ActiveRequests []string
	// MFAVerified is the UUID of an MFA device when this Identity was
	// confirmed immediately after an MFA check.
	MFAVerified string
	// PreviousIdentityExpires is the expiry time of the identity/cert that this
	// identity/cert was derived from. It is used to determine a session's hard
	// deadline in cases where both require_session_mfa and disconnect_expired_cert
	// are enabled. See https://github.com/gravitational/teleport/issues/18544.
	PreviousIdentityExpires time.Time
	// LoginIP is an observed IP of the client on the moment of certificate creation.
	LoginIP string
	// PinnedIP is an IP from which client must communicate with Teleport.
	PinnedIP string
	// DisallowReissue flags that any attempt to request new certificates while
	// authenticated with this cert should be denied.
	DisallowReissue bool
	// CertificateExtensions are user configured ssh key extensions (note: this field also
	// ends up aggregating all *unknown* extensions during cert parsing, meaning that this
	// can sometimes contain fields that were inserted by a newer version of teleport).
	CertificateExtensions []*types.CertExtension
	// Renewable indicates this certificate is renewable.
	Renewable bool
	// Generation counts the number of times a certificate has been renewed, with a generation of 1
	// meaning the cert has never been renewed. A generation of zero means the cert's generation is
	// not being tracked.
	Generation uint64
	// BotName is set to the name of the bot, if the user is a Machine ID bot user.
	// Empty for human users.
	BotName string
	// BotInstanceID is the unique identifier for the bot instance, if this is a
	// Machine ID bot. It is empty for human users.
	BotInstanceID string
	// AllowedResourceIDs lists the resources the user should be able to access.
	AllowedResourceIDs []types.ResourceID
	// ConnectionDiagnosticID references the ConnectionDiagnostic that we should use to append traces when testing a Connection.
	ConnectionDiagnosticID string
	// PrivateKeyPolicy is the private key policy supported by this certificate.
	PrivateKeyPolicy keys.PrivateKeyPolicy
	// DeviceID is the trusted device identifier.
	DeviceID string
	// DeviceAssetTag is the device inventory identifier.
	DeviceAssetTag string
	// DeviceCredentialID is the identifier for the credential used by the device
	// to authenticate itself.
	DeviceCredentialID string
	// GitHubUserID indicates the GitHub user ID identified by the GitHub
	// connector.
	GitHubUserID string
	// GitHubUsername indicates the GitHub username identified by the GitHub
	// connector.
	GitHubUsername string
}

// Encode encodes the identity into an ssh certificate. Note that the returned certificate is incomplete
// and must be have its public key set before signing.
func (i *Identity) Encode(certFormat string) (*ssh.Certificate, error) {
	validBefore := i.ValidBefore
	if validBefore == 0 {
		validBefore = uint64(ssh.CertTimeInfinity)
	}
	validAfter := i.ValidAfter
	if validAfter == 0 {
		validAfter = uint64(time.Now().UTC().Add(-1 * time.Minute).Unix())
	}

	if i.CertType == 0 {
		return nil, trace.BadParameter("cannot encode ssh identity missing required field CertType")
	}

	cert := &ssh.Certificate{
		// we have to use key id to identify teleport user
		KeyId:           i.Username,
		ValidPrincipals: i.Principals,
		ValidAfter:      validAfter,
		ValidBefore:     validBefore,
		CertType:        i.CertType,
	}

	cert.Permissions.Extensions = make(map[string]string)

	if i.CertType == ssh.UserCert {
		cert.Permissions.Extensions[teleport.CertExtensionPermitPTY] = ""
	}

	// --- host extensions ---

	if sr := i.SystemRole.String(); sr != "" {
		cert.Permissions.Extensions[utils.CertExtensionRole] = sr
	}

	if i.ClusterName != "" {
		cert.Permissions.Extensions[utils.CertExtensionAuthority] = i.ClusterName
	}

	// --- user extensions ---

	if i.PermitX11Forwarding {
		cert.Permissions.Extensions[teleport.CertExtensionPermitX11Forwarding] = ""
	}
	if i.PermitAgentForwarding {
		cert.Permissions.Extensions[teleport.CertExtensionPermitAgentForwarding] = ""
	}
	if i.PermitPortForwarding {
		cert.Permissions.Extensions[teleport.CertExtensionPermitPortForwarding] = ""
	}
	if i.MFAVerified != "" {
		cert.Permissions.Extensions[teleport.CertExtensionMFAVerified] = i.MFAVerified
	}
	if !i.PreviousIdentityExpires.IsZero() {
		cert.Permissions.Extensions[teleport.CertExtensionPreviousIdentityExpires] = i.PreviousIdentityExpires.Format(time.RFC3339)
	}
	if i.LoginIP != "" {
		cert.Permissions.Extensions[teleport.CertExtensionLoginIP] = i.LoginIP
	}
	if i.Impersonator != "" {
		cert.Permissions.Extensions[teleport.CertExtensionImpersonator] = i.Impersonator
	}
	if i.DisallowReissue {
		cert.Permissions.Extensions[teleport.CertExtensionDisallowReissue] = ""
	}
	if i.Renewable {
		cert.Permissions.Extensions[teleport.CertExtensionRenewable] = ""
	}
	if i.Generation > 0 {
		cert.Permissions.Extensions[teleport.CertExtensionGeneration] = fmt.Sprint(i.Generation)
	}
	if i.BotName != "" {
		cert.Permissions.Extensions[teleport.CertExtensionBotName] = i.BotName
	}
	if i.BotInstanceID != "" {
		cert.Permissions.Extensions[teleport.CertExtensionBotInstanceID] = i.BotInstanceID
	}
	if len(i.AllowedResourceIDs) != 0 {
		requestedResourcesStr, err := types.ResourceIDsToString(i.AllowedResourceIDs)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cert.Permissions.Extensions[teleport.CertExtensionAllowedResources] = requestedResourcesStr
	}
	if i.ConnectionDiagnosticID != "" {
		cert.Permissions.Extensions[teleport.CertExtensionConnectionDiagnosticID] = i.ConnectionDiagnosticID
	}
	if i.PrivateKeyPolicy != "" {
		cert.Permissions.Extensions[teleport.CertExtensionPrivateKeyPolicy] = string(i.PrivateKeyPolicy)
	}
	if devID := i.DeviceID; devID != "" {
		cert.Permissions.Extensions[teleport.CertExtensionDeviceID] = devID
	}
	if assetTag := i.DeviceAssetTag; assetTag != "" {
		cert.Permissions.Extensions[teleport.CertExtensionDeviceAssetTag] = assetTag
	}
	if credID := i.DeviceCredentialID; credID != "" {
		cert.Permissions.Extensions[teleport.CertExtensionDeviceCredentialID] = credID
	}
	if i.GitHubUserID != "" {
		cert.Permissions.Extensions[teleport.CertExtensionGitHubUserID] = i.GitHubUserID
	}
	if i.GitHubUsername != "" {
		cert.Permissions.Extensions[teleport.CertExtensionGitHubUsername] = i.GitHubUsername
	}

	if i.PinnedIP != "" {
		if cert.CriticalOptions == nil {
			cert.CriticalOptions = make(map[string]string)
		}
		// IPv4, all bits matter
		ip := i.PinnedIP + "/32"
		if strings.Contains(i.PinnedIP, ":") {
			// IPv6
			ip = i.PinnedIP + "/128"
		}
		cert.CriticalOptions[teleport.CertCriticalOptionSourceAddress] = ip
	}

	for _, extension := range i.CertificateExtensions {
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
	if certFormat == constants.CertificateFormatStandard {
		traits, err := wrappers.MarshalTraits(&i.Traits)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(traits) > 0 {
			cert.Permissions.Extensions[teleport.CertExtensionTeleportTraits] = string(traits)
		}
		if len(i.Roles) != 0 {
			roles := certRoles{
				Roles: i.Roles,
			}
			rolesExt, err := roles.Marshal()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			cert.Permissions.Extensions[teleport.CertExtensionTeleportRoles] = string(rolesExt)
		}
		if i.RouteToCluster != "" {
			cert.Permissions.Extensions[teleport.CertExtensionTeleportRouteToCluster] = i.RouteToCluster
		}
		if len(i.ActiveRequests) != 0 {
			reqs := requestIDs{
				AccessRequests: i.ActiveRequests,
			}
			reqsExt, err := reqs.Marshal()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			cert.Permissions.Extensions[teleport.CertExtensionTeleportActiveRequests] = string(reqsExt)
		}
	}

	return cert, nil
}

// GetDeviceMetadata returns information about user's trusted device.
func (i *Identity) GetDeviceMetadata() *apievents.DeviceMetadata {
	if i == nil {
		return nil
	}
	if i.DeviceID == "" && i.DeviceAssetTag == "" && i.DeviceCredentialID == "" {
		return nil
	}

	return &apievents.DeviceMetadata{
		DeviceId:     i.DeviceID,
		AssetTag:     i.DeviceAssetTag,
		CredentialId: i.DeviceCredentialID,
	}
}

// DecodeIdentity decodes an ssh certificate into an identity.
func DecodeIdentity(cert *ssh.Certificate) (*Identity, error) {
	ident := &Identity{
		Username:    cert.KeyId,
		Principals:  cert.ValidPrincipals,
		ValidAfter:  cert.ValidAfter,
		ValidBefore: cert.ValidBefore,
		CertType:    cert.CertType,
	}

	// clone the extension map and remove entries from the clone as they are processed so
	// that we can easily aggregate the remainder into the CertificateExtensions field.
	extensions := maps.Clone(cert.Extensions)

	takeExtension := func(name string) (value string, ok bool) {
		v, ok := extensions[name]
		if !ok {
			return "", false
		}
		delete(extensions, name)
		return v, true
	}

	takeValue := func(name string) string {
		value, _ := takeExtension(name)
		return value
	}

	takeBool := func(name string) bool {
		_, ok := takeExtension(name)
		return ok
	}

	// ignore the permit pty extension, teleport considers this permission implied for all users
	_, _ = takeExtension(teleport.CertExtensionPermitPTY)

	// --- host extensions ---

	if v, ok := takeExtension(utils.CertExtensionRole); ok {
		ident.SystemRole = types.SystemRole(v)
	}

	ident.ClusterName = takeValue(utils.CertExtensionAuthority)

	// --- user extensions ---

	ident.PermitX11Forwarding = takeBool(teleport.CertExtensionPermitX11Forwarding)
	ident.PermitAgentForwarding = takeBool(teleport.CertExtensionPermitAgentForwarding)
	ident.PermitPortForwarding = takeBool(teleport.CertExtensionPermitPortForwarding)
	ident.MFAVerified = takeValue(teleport.CertExtensionMFAVerified)

	if v, ok := takeExtension(teleport.CertExtensionPreviousIdentityExpires); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, trace.BadParameter("failed to parse value %q for extension %q as RFC3339 timestamp: %v", v, teleport.CertExtensionPreviousIdentityExpires, err)
		}
		ident.PreviousIdentityExpires = t
	}

	ident.LoginIP = takeValue(teleport.CertExtensionLoginIP)
	ident.Impersonator = takeValue(teleport.CertExtensionImpersonator)
	ident.DisallowReissue = takeBool(teleport.CertExtensionDisallowReissue)
	ident.Renewable = takeBool(teleport.CertExtensionRenewable)

	if v, ok := takeExtension(teleport.CertExtensionGeneration); ok {
		i, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, trace.BadParameter("failed to parse value %q for extension %q as uint64: %v", v, teleport.CertExtensionGeneration, err)
		}
		ident.Generation = i
	}

	ident.BotName = takeValue(teleport.CertExtensionBotName)
	ident.BotInstanceID = takeValue(teleport.CertExtensionBotInstanceID)

	if v, ok := takeExtension(teleport.CertExtensionAllowedResources); ok {
		resourceIDs, err := types.ResourceIDsFromString(v)
		if err != nil {
			return nil, trace.BadParameter("failed to parse value %q for extension %q as resource IDs: %v", v, teleport.CertExtensionAllowedResources, err)
		}
		ident.AllowedResourceIDs = resourceIDs
	}

	ident.ConnectionDiagnosticID = takeValue(teleport.CertExtensionConnectionDiagnosticID)
	ident.PrivateKeyPolicy = keys.PrivateKeyPolicy(takeValue(teleport.CertExtensionPrivateKeyPolicy))
	ident.DeviceID = takeValue(teleport.CertExtensionDeviceID)
	ident.DeviceAssetTag = takeValue(teleport.CertExtensionDeviceAssetTag)
	ident.DeviceCredentialID = takeValue(teleport.CertExtensionDeviceCredentialID)
	ident.GitHubUserID = takeValue(teleport.CertExtensionGitHubUserID)
	ident.GitHubUsername = takeValue(teleport.CertExtensionGitHubUsername)

	if v, ok := cert.CriticalOptions[teleport.CertCriticalOptionSourceAddress]; ok {
		parts := strings.Split(v, "/")
		if len(parts) != 2 {
			return nil, trace.BadParameter("failed to parse value %q for critical option %q as CIDR", v, teleport.CertCriticalOptionSourceAddress)
		}
		ident.PinnedIP = parts[0]
	}

	if v, ok := takeExtension(teleport.CertExtensionTeleportTraits); ok {
		var traits wrappers.Traits
		if err := wrappers.UnmarshalTraits([]byte(v), &traits); err != nil {
			return nil, trace.BadParameter("failed to unmarshal value %q for extension %q as traits: %v", v, teleport.CertExtensionTeleportTraits, err)
		}
		ident.Traits = traits
	}

	if v, ok := takeExtension(teleport.CertExtensionTeleportRoles); ok {
		var roles certRoles
		if err := roles.Unmarshal([]byte(v)); err != nil {
			return nil, trace.BadParameter("failed to unmarshal value %q for extension %q as roles: %v", v, teleport.CertExtensionTeleportRoles, err)
		}
		ident.Roles = roles.Roles
	}

	ident.RouteToCluster = takeValue(teleport.CertExtensionTeleportRouteToCluster)

	if v, ok := takeExtension(teleport.CertExtensionTeleportActiveRequests); ok {
		var reqs requestIDs
		if err := reqs.Unmarshal([]byte(v)); err != nil {
			return nil, trace.BadParameter("failed to unmarshal value %q for extension %q as active requests: %v", v, teleport.CertExtensionTeleportActiveRequests, err)
		}
		ident.ActiveRequests = reqs.AccessRequests
	}

	// aggregate all remaining extensions into the CertificateExtensions field
	for name, value := range extensions {
		ident.CertificateExtensions = append(ident.CertificateExtensions, &types.CertExtension{
			Name:  name,
			Value: value,
			Type:  types.CertExtensionType_SSH,
			Mode:  types.CertExtensionMode_EXTENSION,
		})
	}

	return ident, nil
}
