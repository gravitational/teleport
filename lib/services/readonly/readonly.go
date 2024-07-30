/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package readonly

import (
	"time"

	protobuf "google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/constants"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
)

// NOTE: is best to avoid importing anything from lib other than lib/utils in this package in
// order to ensure that we can import it anywhere api/types is being used.

// AuthPreference is a read-only subset of types.AuthPreference used on certain hot paths
// to ensure that we do not modify the underlying AuthPreference as it may be shared across
// multiple goroutines.
type AuthPreference interface {
	GetSecondFactor() constants.SecondFactorType
	GetDisconnectExpiredCert() bool
	GetLockingMode() constants.LockingMode
	GetDeviceTrust() *types.DeviceTrust
	GetPrivateKeyPolicy() keys.PrivateKeyPolicy
	IsAdminActionMFAEnforced() bool
	GetRequireMFAType() types.RequireMFAType
	IsSAMLIdPEnabled() bool
	GetDefaultSessionTTL() types.Duration
	GetHardwareKeySerialNumberValidation() (*types.HardwareKeySerialNumberValidation, error)
	GetAllowPasswordless() bool
	Clone() types.AuthPreference
}

type sealedAuthPreference struct {
	AuthPreference
}

// sealAuthPreference returns a read-only version of the AuthPreference.
func sealAuthPreference(p types.AuthPreference) AuthPreference {
	if p == nil {
		// preserving nils simplifies error flow-control
		return nil
	}
	return sealedAuthPreference{AuthPreference: p}
}

// ClusterNetworkingConfig is a read-only subset of types.ClusterNetworkingConfig used on certain hot paths
// to ensure that we do not modify the underlying ClusterNetworkingConfig as it may be shared across
// multiple goroutines.
type ClusterNetworkingConfig interface {
	GetCaseInsensitiveRouting() bool
	GetWebIdleTimeout() time.Duration
	Clone() types.ClusterNetworkingConfig
}

type sealedClusterNetworkingConfig struct {
	ClusterNetworkingConfig
}

// sealClusterNetworkingConfig returns a read-only version of the ClusterNetworkingConfig.
func sealClusterNetworkingConfig(c ClusterNetworkingConfig) ClusterNetworkingConfig {
	if c == nil {
		// preserving nils simplifies error flow-control
		return nil
	}
	return sealedClusterNetworkingConfig{ClusterNetworkingConfig: c}
}

// SessionRecordingConfig is a read-only subset of types.SessionRecordingConfig used on certain hot paths
// to ensure that we do not modify the underlying SessionRecordingConfig as it may be shared across
// multiple goroutines.
type SessionRecordingConfig interface {
	GetMode() string
	GetProxyChecksHostKeys() bool
	Clone() types.SessionRecordingConfig
}

type sealedSessionRecordingConfig struct {
	SessionRecordingConfig
}

// sealSessionRecordingConfig returns a read-only version of the SessionRecordingConfig.
func sealSessionRecordingConfig(c SessionRecordingConfig) SessionRecordingConfig {
	if c == nil {
		// preserving nils simplifies error flow-control
		return nil
	}
	return sealedSessionRecordingConfig{SessionRecordingConfig: c}
}

// AccessGraphSettings is a read-only subset of clusterconfigpb.AccessGraphSettings used on certain hot paths
// to ensure that we do not modify the underlying AccessGraphSettings as it may be shared across
// multiple goroutines.
type AccessGraphSettings interface {
	SecretsScanConfig() clusterconfigpb.AccessGraphSecretsScanConfig
	Clone() *clusterconfigpb.AccessGraphSettings
}

type sealedAccessGraphSettings struct {
	*clusterconfigpb.AccessGraphSettings
}

// sealAccessGraphSettings returns a read-only version of the SessionRecordingConfig.
func sealAccessGraphSettings(c *clusterconfigpb.AccessGraphSettings) AccessGraphSettings {
	if c == nil {
		// preserving nils simplifies error flow-control
		return nil
	}
	return sealedAccessGraphSettings{c}
}

func (a sealedAccessGraphSettings) SecretsScanConfig() clusterconfigpb.AccessGraphSecretsScanConfig {
	return a.GetSpec().GetSecretsScanConfig()
}

func (a sealedAccessGraphSettings) Clone() *clusterconfigpb.AccessGraphSettings {
	return protobuf.Clone(a.AccessGraphSettings).(*clusterconfigpb.AccessGraphSettings)
}
