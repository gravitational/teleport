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

package authz

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	dtconfig "github.com/gravitational/teleport/lib/devicetrust/config"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/tlsca"
)

// ErrTrustedDeviceRequired is returned when access to a resource requires a
// trusted device.
var ErrTrustedDeviceRequired = &trace.AccessDeniedError{
	Message: "access to resource requires a trusted device",
}

// IsTLSDeviceVerified returns true if ext contains all required device
// extensions.
func IsTLSDeviceVerified(ext *tlsca.DeviceExtensions) bool {
	// Expect all device extensions to be present.
	return ext != nil && ext.DeviceID != "" && ext.AssetTag != "" && ext.CredentialID != ""
}

// VerifyTLSUser verifies if the TLS identity has the required extensions to
// fulfill the device trust configuration.
func VerifyTLSUser(ctx context.Context, dt *types.DeviceTrust, identity tlsca.Identity) error {
	return verifyDeviceExtensions(ctx, dt, identity.Username, IsTLSDeviceVerified(&identity.DeviceExtensions))
}

// IsSSHDeviceVerified returns true if cert contains all required device
// extensions.
func IsSSHDeviceVerified(ident *sshca.Identity) bool {
	// Expect all device extensions to be present.
	return ident != nil &&
		ident.DeviceID != "" &&
		ident.DeviceAssetTag != "" &&
		ident.DeviceCredentialID != ""
}

// HasDeviceTrustExtensions returns true if the certificate's extension names
// include all the required device-related extensions.
// Unlike IsSSHDeviceVerified, this function operates on a list of extensions,
// such as those in lib/client.ProfileStatus.Extensions.
func HasDeviceTrustExtensions(extensions []string) bool {
	hasCertExtensionDeviceID := false
	hasCertExtensionDeviceAssetTag := false
	hasCertExtensionDeviceCredentialID := false
	for _, extension := range extensions {
		switch extension {
		case teleport.CertExtensionDeviceID:
			hasCertExtensionDeviceID = true
		case teleport.CertExtensionDeviceAssetTag:
			hasCertExtensionDeviceAssetTag = true
		case teleport.CertExtensionDeviceCredentialID:
			hasCertExtensionDeviceCredentialID = true
		}
	}

	return hasCertExtensionDeviceAssetTag && hasCertExtensionDeviceID && hasCertExtensionDeviceCredentialID
}

// VerifySSHUser verifies if the SSH certificate has the required extensions to
// fulfill the device trust configuration.
func VerifySSHUser(ctx context.Context, dt *types.DeviceTrust, ident *sshca.Identity) error {
	if ident == nil {
		return trace.BadParameter("ssh identity required")
	}

	return verifyDeviceExtensions(ctx, dt, ident.Username, IsSSHDeviceVerified(ident))
}

func verifyDeviceExtensions(ctx context.Context, dt *types.DeviceTrust, username string, verified bool) error {
	mode := dtconfig.GetEnforcementMode(dt)
	switch {
	case mode != constants.DeviceTrustModeRequired:
		return nil // OK, extensions not enforced.
	case !verified:
		slog.DebugContext(ctx, "Device Trust: denied access for unidentified device", "user", username)
		return trace.Wrap(ErrTrustedDeviceRequired)
	default:
		return nil
	}
}
