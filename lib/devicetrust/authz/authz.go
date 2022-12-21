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

// VerifyTLSUser verifies if the TLS identity has the required extensions to
// fulfill the device trust configuration.
func VerifyTLSUser(dt *types.DeviceTrust, identity tlsca.Identity) error {
	return verifyDeviceExtensions(dt, identity.Username, identity.DeviceExtensions)
}

// VerifySSHUser verifies if the SSH certificate has the required extensions to
// fulfill the device trust configuration.
func VerifySSHUser(dt *types.DeviceTrust, cert *ssh.Certificate) error {
	if cert == nil {
		return trace.BadParameter("cert required")
	}

	username := cert.KeyId
	return verifyDeviceExtensions(dt, username, tlsca.DeviceExtensions{
		DeviceID:     cert.Extensions[teleport.CertExtensionDeviceID],
		AssetTag:     cert.Extensions[teleport.CertExtensionDeviceAssetTag],
		CredentialID: cert.Extensions[teleport.CertExtensionDeviceCredentialID],
	})
}

func verifyDeviceExtensions(dt *types.DeviceTrust, username string, ext tlsca.DeviceExtensions) error {
	mode := config.GetEffectiveMode(dt)
	maybeLogModeMismatch(mode, dt)

	if mode != constants.DeviceTrustModeRequired {
		return nil // OK, extensions not enforced.
	}

	// Teleport-issued device certificates always contain all three fields, so we
	// expect either all or none to be present.
	// There's little value in trying to distinguish other situations.
	if ext.DeviceID == "" || ext.AssetTag == "" || ext.CredentialID == "" {
		log.
			WithField("User", username).
			Debug("Device Trust: denied access for unidentified device")
		return trace.AccessDenied("unauthorized device")
	}

	return nil
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
