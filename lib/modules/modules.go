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

// package modules allows external packages override certain behavioral
// aspects of teleport
package modules

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/tlsca"
)

// Features provides supported and unsupported features
type Features struct {
	// --------------- Cloud Settings
	// Cloud enables some cloud-related features
	Cloud bool
	// CustomTheme holds the name of WebUI custom theme.
	CustomTheme string
	// IsStripeManaged indicates if the cluster billing is managed via Stripe
	IsStripeManaged bool
	// IsUsageBasedBilling enables some usage-based billing features
	IsUsageBasedBilling bool
	// Questionnaire indicates whether cluster users should get an onboarding questionnaire
	Questionnaire bool
	// SupportType indicates the type of customer's support
	SupportType proto.SupportType
	// Entitlements reflect Cloud Entitlements including access and limits
	Entitlements map[entitlements.EntitlementKind]EntitlementInfo
	// CloudAnonymizationKey is the key used to anonymize usage events in a cluster.
	// Only applicable for Cloud customers (self-hosted clusters get their anonymization key from the
	// license file).
	CloudAnonymizationKey []byte

	// todo (michellescripts) have the following fields evaluated for deprecation, consolidation, or fetch from Cloud
	// AdvancedAccessWorkflows is currently set to the value of the Cloud Access Requests entitlement
	AdvancedAccessWorkflows bool
	// RecoveryCodes enables account recovery codes
	RecoveryCodes bool
	// Plugins enables hosted plugins
	Plugins bool
	// AutomaticUpgrades enables automatic upgrades of agents/services.
	AutomaticUpgrades bool
	// AccessGraph enables the usage of access graph.
	// NOTE: this is a legacy flag that is currently used to signal
	// that Access Graph integration is *enabled* on a cluster.
	// *Access* to the feature is gated on the `Policy` flag.
	// TODO(justinas): remove this field once "TAG enabled" status is moved to a resource in the backend.
	AccessGraph bool
	// AccessMonitoringConfigured contributes to the enablement of access monitoring.
	// NOTE: this flag is used to signal that Access Monitoring is *enabled* on a cluster.
	// *Access* to the feature is gated on the `AccessMonitoring` entitlement.
	AccessMonitoringConfigured bool
	// --------------- Deprecated Fields
	// AccessControls enables FIPS access controls
	// Deprecated
	AccessControls bool
	// Assist enables Assistant feature
	// Deprecated
	Assist bool
	// ProductType describes the product being used.
	// Deprecated
	ProductType ProductType
}

// EntitlementInfo is the state and limits of a particular entitlement
type EntitlementInfo struct {
	// Enabled indicates the feature is 'on' if true; feature is disabled if false
	Enabled bool
	// Limit indicates the allotted amount of use when limited; if 0 use is unlimited
	Limit int32
}

// UnderLimit tests that a given entitlement is under its prescribed limit
// based on the supplied use count. A return value of `true` indicates that
// there is still at least *some* capacity left in the entitlement. The actual
// definition of a "use" depends on the entitlement in question.
// A disabled entitlement is always out of its limit.
func (e EntitlementInfo) UnderLimit(count int) bool {
	return e.Enabled && (e.Limit == 0 || count < int(e.Limit))
}

// ToProto converts Features into proto.Features
func (f Features) ToProto() *proto.Features {
	protoF := &proto.Features{
		Cloud:                      f.Cloud,
		CustomTheme:                f.CustomTheme,
		IsStripeManaged:            f.IsStripeManaged,
		IsUsageBased:               f.IsUsageBasedBilling,
		Questionnaire:              f.Questionnaire,
		SupportType:                f.SupportType,
		AccessControls:             f.AccessControls,
		AccessGraph:                f.AccessGraph,
		AdvancedAccessWorkflows:    f.AdvancedAccessWorkflows,
		AutomaticUpgrades:          f.AutomaticUpgrades,
		Plugins:                    f.Plugins,
		ProductType:                proto.ProductType(f.ProductType),
		RecoveryCodes:              f.RecoveryCodes,
		AccessMonitoringConfigured: f.AccessMonitoringConfigured,
		Entitlements:               f.EntitlementsToProto(),
		CloudAnonymizationKey:      f.CloudAnonymizationKey,
	}

	// remove setLegacyLogic in v18
	setLegacyLogic(protoF, f)
	return protoF
}

// setLegacyLogic sets the deprecated fields; to be removed in v18 - use entitlements
func setLegacyLogic(protoF *proto.Features, f Features) {
	protoF.Kubernetes = f.GetEntitlement(entitlements.K8s).Enabled
	protoF.App = f.GetEntitlement(entitlements.App).Enabled
	protoF.DB = f.GetEntitlement(entitlements.DB).Enabled
	protoF.OIDC = f.GetEntitlement(entitlements.OIDC).Enabled
	protoF.SAML = f.GetEntitlement(entitlements.SAML).Enabled
	protoF.HSM = f.GetEntitlement(entitlements.HSM).Enabled
	protoF.Desktop = f.GetEntitlement(entitlements.Desktop).Enabled
	protoF.FeatureHiding = f.GetEntitlement(entitlements.FeatureHiding).Enabled
	protoF.IdentityGovernance = f.GetEntitlement(entitlements.Identity).Enabled
	protoF.ExternalAuditStorage = f.GetEntitlement(entitlements.ExternalAuditStorage).Enabled
	protoF.JoinActiveSessions = f.GetEntitlement(entitlements.JoinActiveSessions).Enabled
	protoF.MobileDeviceManagement = f.GetEntitlement(entitlements.MobileDeviceManagement).Enabled

	protoF.DeviceTrust = &proto.DeviceTrustFeature{
		Enabled: f.GetEntitlement(entitlements.DeviceTrust).Enabled, DevicesUsageLimit: f.GetEntitlement(entitlements.DeviceTrust).Limit,
	}
	protoF.AccessRequests = &proto.AccessRequestsFeature{
		MonthlyRequestLimit: f.GetEntitlement(entitlements.AccessRequests).Limit,
	}
	protoF.AccessMonitoring = &proto.AccessMonitoringFeature{
		Enabled: f.AccessMonitoringConfigured, MaxReportRangeLimit: f.GetEntitlement(entitlements.AccessMonitoring).Limit,
	}
	protoF.AccessList = &proto.AccessListFeature{
		CreateLimit: f.GetEntitlement(entitlements.AccessLists).Limit,
	}
	protoF.Policy = &proto.PolicyFeature{
		Enabled: f.GetEntitlement(entitlements.Policy).Enabled,
	}
}

// EntitlementsToProto takes the features.Entitlements object and returns a proto version. If not present on Features, the
// proto entitlement will default to false
func (f Features) EntitlementsToProto() map[string]*proto.EntitlementInfo {
	all := entitlements.AllEntitlements
	result := make(map[string]*proto.EntitlementInfo, len(all))

	for _, e := range all {
		al, ok := f.Entitlements[e]
		if !ok {
			result[string(e)] = &proto.EntitlementInfo{}
			continue
		}

		result[string(e)] = &proto.EntitlementInfo{
			Enabled: al.Enabled,
			Limit:   al.Limit,
		}
	}

	return result
}

// GetEntitlement takes an entitlement and returns either the Features entitlement, or if not present, a false entitlement
func (f Features) GetEntitlement(e entitlements.EntitlementKind) EntitlementInfo {
	al, ok := f.Entitlements[e]
	if !ok {
		return EntitlementInfo{}
	}

	return EntitlementInfo{
		Enabled: al.Enabled,
		Limit:   al.Limit,
	}
}

// GetProtoEntitlement takes a proto features set and an entitlement and returns either the proto features entitlement,
// or if not present, a false entitlement
func GetProtoEntitlement(f *proto.Features, e entitlements.EntitlementKind) *proto.EntitlementInfo {
	fE := f.GetEntitlements()
	al, ok := fE[string(e)]
	if !ok {
		return &proto.EntitlementInfo{}
	}

	return &proto.EntitlementInfo{
		Enabled: al.Enabled,
		Limit:   al.Limit,
	}
}

// ProductType is the type of product.
type ProductType int32

const (
	ProductTypeUnknown ProductType = 0
	// ProductTypeTeam is Teleport ProductTypeTeam product.
	ProductTypeTeam ProductType = 1
	// ProductTypeEUB is Teleport Enterprise Usage Based product.
	ProductTypeEUB ProductType = 2
)

// AccessResourcesGetter is a minimal interface that is used to get access lists
// and related resources from the backend.
type AccessResourcesGetter interface {
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)

	GetAccessList(context.Context, string) (*accesslist.AccessList, error)
	GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error)

	ListAccessListMembers(ctx context.Context, accessList string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)

	GetUser(ctx context.Context, userName string, withSecrets bool) (types.User, error)
	GetRole(ctx context.Context, name string) (types.Role, error)

	GetLock(ctx context.Context, name string) (types.Lock, error)
	GetLocks(ctx context.Context, inForceOnly bool, targets ...types.LockTarget) ([]types.Lock, error)
}

type AccessListSuggestionClient interface {
	GetUser(ctx context.Context, userName string, withSecrets bool) (types.User, error)
	RoleGetter

	GetAccessRequestAllowedPromotions(ctx context.Context, req types.AccessRequest) (*types.AccessRequestAllowedPromotions, error)
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
}

type RoleGetter interface {
	GetRole(ctx context.Context, name string) (types.Role, error)
}

type AccessListAndMembersGetter interface {
	GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error)
	GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error)
	GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error)
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
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
	// BuildType returns build type (OSS, Community or Enterprise)
	BuildType() string
	// IsEnterpriseBuild returns if the binary was built with enterprise modules
	IsEnterpriseBuild() bool
	// IsOSSBuild returns if the binary was built without enterprise modules
	IsOSSBuild() bool
	// AttestHardwareKey attests a hardware key and returns its associated private key policy.
	AttestHardwareKey(context.Context, interface{}, *keys.AttestationStatement, crypto.PublicKey, time.Duration) (*keys.AttestationData, error)
	// GenerateAccessRequestPromotions generates a list of valid promotions for given access request.
	GenerateAccessRequestPromotions(context.Context, AccessResourcesGetter, types.AccessRequest) (*types.AccessRequestAllowedPromotions, error)
	// GetSuggestedAccessLists generates a list of valid promotions for given access request.
	GetSuggestedAccessLists(ctx context.Context, identity *tlsca.Identity, clt AccessListSuggestionClient, accessListGetter AccessListAndMembersGetter, requestID string) ([]*accesslist.AccessList, error)
	// EnableRecoveryCodes enables the usage of recovery codes for resetting forgotten passwords
	EnableRecoveryCodes()
	// EnablePlugins enables the hosted plugins runtime
	EnablePlugins()
	// EnableAccessGraph enables the usage of access graph.
	EnableAccessGraph()
	// EnableAccessMonitoring enables the usage of access monitoring.
	EnableAccessMonitoring()
	// LicenseExpiry returns the expiry date of the enterprise license, if applicable.
	LicenseExpiry() time.Time
}

const (
	// BuildOSS specifies open source build type
	BuildOSS = "oss"
	// BuildEnterprise specifies enterprise build type
	BuildEnterprise = "ent"
	// BuildCommunity identifies builds of Teleport Community Edition,
	// which are distributed on goteleport.com/download under our
	// Teleport Community license agreement.
	BuildCommunity = "community"
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

var ErrCannotDisableSecondFactor = errors.New("cannot disable multi-factor authentication")

// ValidateResource performs additional resource checks.
func ValidateResource(res types.Resource) error {
	// todo(tross): DELETE WHEN ABLE TO [remove env var, leave insecure test mode]
	if GetModules().Features().Cloud ||
		(os.Getenv(teleport.EnvVarAllowNoSecondFactor) != "yes" && !IsInsecureTestMode()) {

		switch r := res.(type) {
		case types.AuthPreference:
			if !r.IsSecondFactorEnforced() {
				return trace.Wrap(ErrCannotDisableSecondFactor)
			}
		}
	}

	// All checks below are Cloud-specific.
	if !GetModules().Features().Cloud {
		return nil
	}

	switch r := res.(type) {
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

var teleportBuildType = BuildOSS

// BuildType returns build type (OSS, Community or Enterprise)
func (p *defaultModules) BuildType() string {
	return teleportBuildType
}

// IsEnterpriseBuild returns false for [defaultModules].
func (p *defaultModules) IsEnterpriseBuild() bool {
	return false
}

// IsOSSBuild returns true for [defaultModules].
func (p *defaultModules) IsOSSBuild() bool {
	return true
}

// PrintVersion prints the Teleport version.
func (p *defaultModules) PrintVersion() {
	fmt.Printf("Teleport v%s git:%s %s\n", teleport.Version, teleport.Gitref, runtime.Version())
}

// LicenseExpiry returns the expiry date of the enterprise license, if applicable.
// Returns the zero value for time.Time for OSS.
func (p *defaultModules) LicenseExpiry() time.Time {
	return time.Time{}
}

// Features returns supported features for default modules which is applied for OSS users
// todo (michellescripts) remove deprecated features
func (p *defaultModules) Features() Features {
	p.loadDynamicValues.Do(func() {
		p.automaticUpgrades = automaticupgrades.IsEnabled()
	})

	return Features{
		AutomaticUpgrades: p.automaticUpgrades,
		SupportType:       proto.SupportType_SUPPORT_TYPE_FREE,
		Entitlements: map[entitlements.EntitlementKind]EntitlementInfo{
			entitlements.App:                {Enabled: true, Limit: 0},
			entitlements.DB:                 {Enabled: true, Limit: 0},
			entitlements.Desktop:            {Enabled: true, Limit: 0},
			entitlements.JoinActiveSessions: {Enabled: true, Limit: 0},
			entitlements.K8s:                {Enabled: true, Limit: 0},
		},
	}
}

// SetFeatures sets features queried from Cloud.
// This is a noop since OSS teleport does not support enterprise features
func (p *defaultModules) SetFeatures(f Features) {
}

func (p *defaultModules) IsBoringBinary() bool {
	return IsBoringBinary()
}

// AttestHardwareKey attests a hardware key.
func (p *defaultModules) AttestHardwareKey(_ context.Context, _ interface{}, _ *keys.AttestationStatement, _ crypto.PublicKey, _ time.Duration) (*keys.AttestationData, error) {
	// Default modules do not support attesting hardware keys.
	return nil, trace.NotFound("no attestation data for the given key")
}

// GenerateAccessRequestPromotions is a noop since OSS teleport does not support generating access list promotions.
func (p *defaultModules) GenerateAccessRequestPromotions(_ context.Context, _ AccessResourcesGetter, _ types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	// The default module does not support generating access list promotions.
	return types.NewAccessRequestAllowedPromotions(nil), nil
}

func (p *defaultModules) GetSuggestedAccessLists(ctx context.Context, identity *tlsca.Identity, clt AccessListSuggestionClient,
	accessListGetter AccessListAndMembersGetter, requestID string,
) ([]*accesslist.AccessList, error) {
	return nil, trace.NotImplemented("GetSuggestedAccessLists not implemented")
}

// EnableRecoveryCodes enables recovery codes. This is a noop since OSS teleport does not
// support recovery codes
func (p *defaultModules) EnableRecoveryCodes() {
}

// EnablePlugins enables hosted plugins runtime.
// This is a noop since OSS teleport does not support hosted plugins
func (p *defaultModules) EnablePlugins() {
}

// EnableAccessGraph enables the usage of access graph.
// This is a noop since OSS teleport does not support access graph.
func (p *defaultModules) EnableAccessGraph() {}

// EnableAccessMonitoring enables the usage of access monitoring.
// This is a noop since OSS teleport does not support access monitoring.
func (p *defaultModules) EnableAccessMonitoring() {}

var (
	mutex   sync.Mutex
	modules Modules = &defaultModules{}
)

var (
	// flagLock protects access to accessing insecure test mode below
	flagLock sync.Mutex

	// insecureTestAllow is used to allow disabling second factor auth
	// in test environments. Not user configurable.
	insecureTestAllowNoSecondFactor bool
)

// SetInsecureTestMode is used to set insecure test mode on, to allow
// second factor to be disabled
func SetInsecureTestMode(m bool) {
	flagLock.Lock()
	defer flagLock.Unlock()
	insecureTestAllowNoSecondFactor = m
}

// IsInsecureTestMode retrieves the current insecure test mode value
func IsInsecureTestMode() bool {
	flagLock.Lock()
	defer flagLock.Unlock()
	return insecureTestAllowNoSecondFactor
}
