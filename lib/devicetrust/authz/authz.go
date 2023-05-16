// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		return trace.AccessDenied("unauthorized device")
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
