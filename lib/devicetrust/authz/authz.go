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
	"sync"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/devicetrust/config"
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
func VerifyTLSUser(dt *types.DeviceTrust, identity tlsca.Identity) error {
	return verifyDeviceExtensions(dt, identity.Username, IsTLSDeviceVerified(&identity.DeviceExtensions))
}

// IsSSHDeviceVerified returns true if cert contains all required device
// extensions.
func IsSSHDeviceVerified(cert *ssh.Certificate) bool {
	// Expect all device extensions to be present.
	return cert != nil &&
		cert.Extensions[teleport.CertExtensionDeviceID] != "" &&
		cert.Extensions[teleport.CertExtensionDeviceAssetTag] != "" &&
		cert.Extensions[teleport.CertExtensionDeviceCredentialID] != ""
}

// VerifySSHUser verifies if the SSH certificate has the required extensions to
// fulfill the device trust configuration.
func VerifySSHUser(dt *types.DeviceTrust, cert *ssh.Certificate) error {
	if cert == nil {
		return trace.BadParameter("cert required")
	}

	username := cert.KeyId
	return verifyDeviceExtensions(dt, username, IsSSHDeviceVerified(cert))
}

func verifyDeviceExtensions(dt *types.DeviceTrust, username string, verified bool) error {
	mode := config.GetEffectiveMode(dt)
	maybeLogModeMismatch(mode, dt)

	switch {
	case mode != constants.DeviceTrustModeRequired:
		return nil // OK, extensions not enforced.
	case !verified:
		log.
			WithField("User", username).
			Debug("Device Trust: denied access for unidentified device")
		return trace.Wrap(ErrTrustedDeviceRequired)
	default:
		return nil
	}
}

var logModeOnce sync.Once

func maybeLogModeMismatch(effective string, dt *types.DeviceTrust) {
	if dt == nil || dt.Mode == "" || effective == dt.Mode {
		return
	}

	logModeOnce.Do(func() {
		log.Warnf("Device Trust: mode %q requires Teleport Enterprise. Using effective mode %q.", dt.Mode, effective)
	})
}
