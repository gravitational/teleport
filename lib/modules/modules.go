/*
Copyright 2017-2021 Gravitational, Inc.

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

// package modules allows external packages override certain behavioral
// aspects of teleport
package modules

import (
	"context"
	"crypto"
	"crypto/sha256"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/automaticupgrades"
)

// Features provides supported and unsupported features
type Features struct {
	// Kubernetes enables Kubernetes Access product
	Kubernetes bool
	// App enables Application Access product
	App bool
	// DB enables database access product
	DB bool
	// OIDC enables OIDC connectors
	OIDC bool
	// SAML enables SAML connectors
	SAML bool
	// AccessControls enables FIPS access controls
	AccessControls bool
	// AdvancedAccessWorkflows enables advanced access workflows
	AdvancedAccessWorkflows bool
	// Cloud enables some cloud-related features
	Cloud bool
	// HSM enables PKCS#11 HSM support
	HSM bool
	// Desktop enables desktop access product
	Desktop bool
	// RecoveryCodes enables account recovery codes
	RecoveryCodes bool
	// Plugins enables hosted plugins
	Plugins bool
	// AutomaticUpgrades enables automatic upgrades of agents/services.
	AutomaticUpgrades bool
	// IsUsageBasedBilling enables some usage-based billing features
	IsUsageBasedBilling bool
	// Assist enables Assistant feature
	Assist bool
	// DeviceTrust holds its namesake feature settings.
	DeviceTrust DeviceTrustFeature
}

// DeviceTrustFeature holds the Device Trust feature general and usage-based
// settings.
// Requires Teleport Enterprise.
type DeviceTrustFeature struct {
	// Enabled is true if the Device Trust feature is enabled.
	Enabled bool
	// DevicesUsageLimit is the usage-based limit for the number of
	// registered/enrolled devices, at the implementation's discretion.
	// Meant for usage-based accounts, like Teleport Team. Has no effect if
	// [Features.IsUsageBasedBilling] is `false`.
	DevicesUsageLimit int
}

// ToProto converts Features into proto.Features
func (f Features) ToProto() *proto.Features {
	return &proto.Features{
		Kubernetes:              f.Kubernetes,
		App:                     f.App,
		DB:                      f.DB,
		OIDC:                    f.OIDC,
		SAML:                    f.SAML,
		AccessControls:          f.AccessControls,
		AdvancedAccessWorkflows: f.AdvancedAccessWorkflows,
		Cloud:                   f.Cloud,
		HSM:                     f.HSM,
		Desktop:                 f.Desktop,
		RecoveryCodes:           f.RecoveryCodes,
		Plugins:                 f.Plugins,
		AutomaticUpgrades:       f.AutomaticUpgrades,
		IsUsageBased:            f.IsUsageBasedBilling,
		Assist:                  f.Assist,
		DeviceTrust: &proto.DeviceTrustFeature{
			Enabled:           f.DeviceTrust.Enabled,
			DevicesUsageLimit: int32(f.DeviceTrust.DevicesUsageLimit),
		},
	}
}

// Modules defines interface that external libraries can implement customizing
// default teleport behavior
type Modules interface {
	// PrintVersion prints teleport version
	PrintVersion()
	// IsBoringBinary checks if the binary was compiled with BoringCrypto.
	IsBoringBinary() bool
	// Features returns supported features
	Features() Features
	// SetFeatures set features queried from Cloud
	SetFeatures(Features)
	// BuildType returns build type (OSS or Enterprise)
	BuildType() string
	// AttestHardwareKey attests a hardware key and returns its associated private key policy.
	AttestHardwareKey(context.Context, interface{}, keys.PrivateKeyPolicy, *keys.AttestationStatement, crypto.PublicKey, time.Duration) (keys.PrivateKeyPolicy, error)
	// EnableRecoveryCodes enables the usage of recovery codes for resetting forgotten passwords
	EnableRecoveryCodes()
	// EnablePlugins enables the hosted plugins runtime
	EnablePlugins()
}

const (
	// BuildOSS specifies open source build type
	BuildOSS = "oss"
	// BuildEnterprise specifies enterprise build type
	BuildEnterprise = "ent"
)

// SetModules sets the modules interface
func SetModules(m Modules) {
	mutex.Lock()
	defer mutex.Unlock()
	modules = m
}

// GetModules returns the modules interface
func GetModules() Modules {
	mutex.Lock()
	defer mutex.Unlock()
	return modules
}

// ValidateResource performs additional resource checks.
func ValidateResource(res types.Resource) error {
	// All checks below are Cloud-specific.
	if !GetModules().Features().Cloud {
		return nil
	}

	switch r := res.(type) {
	case types.AuthPreference:
		switch r.GetSecondFactor() {
		case constants.SecondFactorOff, constants.SecondFactorOptional:
			return trace.BadParameter("cannot disable two-factor authentication on Cloud")
		}
	case types.SessionRecordingConfig:
		switch r.GetMode() {
		case types.RecordAtProxy, types.RecordAtProxySync:
			return trace.BadParameter("cannot set proxy recording mode on Cloud")
		}
		if !r.GetProxyChecksHostKeys() {
			return trace.BadParameter("cannot disable strict host key checking on Cloud")
		}
	}
	return nil
}

type defaultModules struct {
	automaticUpgrades bool
	loadDynamicValues sync.Once
}

// BuildType returns build type (OSS or Enterprise)
func (p *defaultModules) BuildType() string {
	return BuildOSS
}

// PrintVersion prints the Teleport version.
func (p *defaultModules) PrintVersion() {
	fmt.Printf("Teleport v%s git:%s %s\n", teleport.Version, teleport.Gitref, runtime.Version())
}

// Features returns supported features
func (p *defaultModules) Features() Features {
	p.loadDynamicValues.Do(func() {
		p.automaticUpgrades = automaticupgrades.IsEnabled()
	})

	return Features{
		Kubernetes:        true,
		DB:                true,
		App:               true,
		Desktop:           true,
		AutomaticUpgrades: p.automaticUpgrades,
		Assist:            true,
	}
}

// SetFeatures sets features queried from Cloud.
// This is a noop since OSS teleport does not support enterprise features
func (p *defaultModules) SetFeatures(f Features) {
}

func (p *defaultModules) IsBoringBinary() bool {
	// Check the package name for one of the boring primitives, if the package
	// path is from BoringCrypto, we know this binary was compiled against the
	// dev.boringcrypto branch of Go.
	hash := sha256.New()
	return reflect.TypeOf(hash).Elem().PkgPath() == "crypto/internal/boring"
}

// AttestHardwareKey attests a hardware key.
func (p *defaultModules) AttestHardwareKey(_ context.Context, _ interface{}, _ keys.PrivateKeyPolicy, _ *keys.AttestationStatement, _ crypto.PublicKey, _ time.Duration) (keys.PrivateKeyPolicy, error) {
	// Default modules do not support attesting hardware keys.
	return keys.PrivateKeyPolicyNone, nil
}

// EnableRecoveryCodes enables recovery codes. This is a noop since OSS teleport does not
// support recovery codes
func (p *defaultModules) EnableRecoveryCodes() {
}

// EnablePlugins enables hosted plugins runtime.
// This is a noop since OSS teleport does not support hosted plugins
func (p *defaultModules) EnablePlugins() {
}

var (
	mutex   sync.Mutex
	modules Modules = &defaultModules{}
)
