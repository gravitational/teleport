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
func VerifyTLSUser(ctx context.Context, dt *types.DeviceTrust, id tlsca.Identity) error {
	return verifyDeviceExtensions(ctx,
		dt,
		id.Username,
		VerifyTrustedDeviceModeParams{
			IsTrustedDevice: IsTLSDeviceVerified(&id.DeviceExtensions),
			IsBot:           id.IsBot(),
		})
}

// IsSSHDeviceVerified returns true if cert contains all required device
// extensions.
func IsSSHDeviceVerified(id *sshca.Identity) bool {
	// Expect all device extensions to be present.
	return id != nil &&
		id.DeviceID != "" &&
		id.DeviceAssetTag != "" &&
		id.DeviceCredentialID != ""
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
func VerifySSHUser(ctx context.Context, dt *types.DeviceTrust, id *sshca.Identity) error {
	if id == nil {
		return trace.BadParameter("ssh identity required")
	}
	return verifyDeviceExtensions(ctx,
		dt,
		id.Username,
		VerifyTrustedDeviceModeParams{
			IsTrustedDevice: IsSSHDeviceVerified(id),
			IsBot:           id.IsBot(),
		})
}

func verifyDeviceExtensions(
	ctx context.Context,
	dt *types.DeviceTrust,
	username string,
	params VerifyTrustedDeviceModeParams,
) error {
	enforcementMode := dtconfig.GetEnforcementMode(dt)

	if err := VerifyTrustedDeviceMode(enforcementMode, params); err != nil {
		slog.DebugContext(ctx, "Device Trust: denied access for unidentified device", "user", username)
		return trace.Wrap(err)
	}

	return nil
}

// VerifyTrustedDeviceModeParams holds additional parameters for
// [VerifyTrustedDeviceMode].
type VerifyTrustedDeviceModeParams struct {
	// IsTrustedDevice informs if the device in use is trusted.
	IsTrustedDevice bool
	// IsBot informs if the user is a bot.
	IsBot bool
	// AllowEmptyMode allows an empty "enforcementMode", treating it similarly to
	// DeviceTrustModeOff.
	AllowEmptyMode bool
}

// VerifyTrustedDeviceMode runs the fundamental device trust authorization
// logic, checking an effective device trust mode against a set of access
// params.
//
// Most callers should use a higher level function, such as [VerifyTLSUser] or
// [VerifySSHUser].
//
// If enforcementMode comes from the global config it must be resolved via
// [dtconfig.GetEnforcementMode] prior to calling the method.
//
// Returns an error, typically ErrTrustedDeviceRequired, if the checked device
// is not allowed.
func VerifyTrustedDeviceMode(
	enforcementMode constants.DeviceTrustMode,
	params VerifyTrustedDeviceModeParams,
) error {
	if enforcementMode == "" && params.AllowEmptyMode {
		return nil // Equivalent to mode=off.
	}

	// Assume required so it denies by default.
	required := true

	// Switch on mode before any exemptions so we catch unknown modes.
	switch enforcementMode {
	case constants.DeviceTrustModeOff, constants.DeviceTrustModeOptional:
		// OK, extensions not enforced.
		required = false

	case constants.DeviceTrustModeRequiredForHumans:
		// Humans must use trusted devices, bots can use untrusted devices.
		required = !params.IsBot

	case constants.DeviceTrustModeRequired:
		// Only trusted devices allowed for bot human and bot users.

	default:
		slog.WarnContext(context.Background(),
			"Unknown device trust mode, treating device as untrusted",
			"mode", enforcementMode,
		)
		return trace.Wrap(ErrTrustedDeviceRequired)
	}

	if required && !params.IsTrustedDevice {
		return trace.Wrap(ErrTrustedDeviceRequired)
	}

	return nil
}
