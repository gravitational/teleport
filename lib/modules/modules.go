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
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/tlsca"
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
	// Currently this flag is to gate actions from OSS clusters.
	//
	// Determining support for access request is currently determined by:
	//   1) Enterprise + [Features.IdentityGovernanceSecurity] == true, new flag
	//   introduced with Enterprise Usage Based (EUB) product.
	//   2) Enterprise + [Features.IsUsageBasedBilling] == false, legacy support
	//   where before EUB, it was unlimited.
	//
	// AdvancedAccessWorkflows is currently set to true for all
	// enterprise editions (team, cloud, on-prem). Historically, access request
	// was only available for enterprise cloud and enterprise on-prem.
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
	// FeatureHiding enables hiding features from being discoverable for users who don't have the necessary permissions.
	FeatureHiding bool
	// AccessRequests holds its namesake feature settings.
	AccessRequests AccessRequestsFeature
	// CustomTheme holds the name of WebUI custom theme.
	CustomTheme string

	// AccessGraph enables the usage of access graph.
	// NOTE: this is a legacy flag that is currently used to signal
	// that Access Graph integration is *enabled* on a cluster.
	// *Access* to the feature is gated on the `Policy` flag.
	// TODO(justinas): remove this field once "TAG enabled" status is moved to a resource in the backend.
	AccessGraph bool
	// IdentityGovernanceSecurity indicates whether IGS related features are enabled:
	// access list, access request, access monitoring, device trust.
	IdentityGovernanceSecurity bool
	// AccessList holds its namesake feature settings.
	AccessList AccessListFeature
	// AccessMonitoring holds its namesake feature settings.
	AccessMonitoring AccessMonitoringFeature
	// ProductType describes the product being used.
	ProductType ProductType
	// Policy holds settings for the Teleport Policy feature set.
	// At the time of writing, this includes Teleport Access Graph (TAG).
	Policy PolicyFeature
	// Questionnaire indicates whether cluster users should get an onboarding questionnaire
	Questionnaire bool
	// IsStripeManaged indicates if the cluster billing is managed via Stripe
	IsStripeManaged bool
	// ExternalAuditStorage indicates whether the EAS feature is enabled in the cluster.
	ExternalAuditStorage bool
	// SupportType indicates the type of customer's support
	SupportType proto.SupportType
	// JoinActiveSessions indicates whether joining active sessions via web UI is enabled
	JoinActiveSessions bool
	// MobileDeviceManagement indicates whether endpoints management (like Jamf Plugin) can be used in the cluster
	MobileDeviceManagement bool
}

// DeviceTrustFeature holds the Device Trust feature general and usage-based
// settings.
// Limits have no affect if [Feature.IdentityGovernanceSecurity] is enabled.
type DeviceTrustFeature struct {
	// Currently this flag is to gate actions from OSS clusters.
	//
	// Determining support for device trust is currently determined by:
	//   1) Enterprise + [Features.IdentityGovernanceSecurity] == true, new flag
	//   introduced with Enterprise Usage Based (EUB) product.
	//   2) Enterprise + [Features.IsUsageBasedBilling] == false, legacy support
	//   where before EUB, it was unlimited.
	Enabled bool
	// DevicesUsageLimit is the usage-based limit for the number of
	// registered/enrolled devices, at the implementation's discretion.
	DevicesUsageLimit int
}

// AccessRequestsFeature holds the Access Requests feature general and usage-based settings.
// Limits have no affect if [Feature.IdentityGovernanceSecurity] is enabled.
type AccessRequestsFeature struct {
	// MonthlyRequestLimit is the usage-based limit for the number of
	// access requests created in a calendar month.
	MonthlyRequestLimit int
}

// AccessListFeature holds the Access List feature settings.
// Limits have no affect if feature is enabled.
type AccessListFeature struct {
	// Limit for the number of access list creatable when feature is
	// not enabled.
	CreateLimit int
}

// AccessMonitoring holds the Access Monitoring feature settings.
// Limits have no affect if [Feature.IdentityGovernanceSecurity] is enabled.
type AccessMonitoringFeature struct {
	// True if enabled in the auth service config: [auth_service.access_monitoring.enabled].
	Enabled bool
	// Defines the max number of days to include in an access report.
	MaxReportRangeLimit int
}

type PolicyFeature struct {
	// Enabled is set to `true` if Teleport Policy is enabled in the license.
	Enabled bool
}

// ToProto converts Features into proto.Features
func (f Features) ToProto() *proto.Features {
	return &proto.Features{
		ProductType:             proto.ProductType(f.ProductType),
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
		FeatureHiding:           f.FeatureHiding,
		CustomTheme:             f.CustomTheme,
		AccessGraph:             f.AccessGraph,
		DeviceTrust: &proto.DeviceTrustFeature{
			Enabled:           f.DeviceTrust.Enabled,
			DevicesUsageLimit: int32(f.DeviceTrust.DevicesUsageLimit),
		},
		AccessRequests: &proto.AccessRequestsFeature{
			MonthlyRequestLimit: int32(f.AccessRequests.MonthlyRequestLimit),
		},
		IdentityGovernance: f.IdentityGovernanceSecurity,
		AccessMonitoring: &proto.AccessMonitoringFeature{
			Enabled:             f.AccessMonitoring.Enabled,
			MaxReportRangeLimit: int32(f.AccessMonitoring.MaxReportRangeLimit),
		},
		AccessList: &proto.AccessListFeature{
			CreateLimit: int32(f.AccessList.CreateLimit),
		},
		Policy: &proto.PolicyFeature{
			Enabled: f.Policy.Enabled,
		},
		Questionnaire:          f.Questionnaire,
		IsStripeManaged:        f.IsStripeManaged,
		ExternalAuditStorage:   f.ExternalAuditStorage,
		SupportType:            f.SupportType,
		JoinActiveSessions:     f.JoinActiveSessions,
		MobileDeviceManagement: f.MobileDeviceManagement,
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

// IsLegacy describes the legacy enterprise product that existed before the
// usage-based product was introduced. Some features (Device Trust, for example)
// require the IGS add-on in usage-based products but are included for legacy
// licenses.
func (f Features) IsLegacy() bool {
	return !f.IsUsageBasedBilling
}

func (f Features) IGSEnabled() bool {
	return f.IdentityGovernanceSecurity
}

// TODO(mcbattirola): remove isTeam when it is no longer used
func (f Features) IsTeam() bool {
	return f.ProductType == ProductTypeTeam
}

// AccessResourcesGetter is a minimal interface that is used to get access lists
// and related resources from the backend.
type AccessResourcesGetter interface {
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error)

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
type AccessListGetter interface {
	GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error)
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
	AttestHardwareKey(context.Context, interface{}, *keys.AttestationStatement, crypto.PublicKey, time.Duration) (*keys.AttestationData, error)
	// GenerateAccessRequestPromotions generates a list of valid promotions for given access request.
	GenerateAccessRequestPromotions(context.Context, AccessResourcesGetter, types.AccessRequest) (*types.AccessRequestAllowedPromotions, error)
	// GetSuggestedAccessLists generates a list of valid promotions for given access request.
	GetSuggestedAccessLists(ctx context.Context, identity *tlsca.Identity, clt AccessListSuggestionClient, accessListGetter AccessListGetter, requestID string) ([]*accesslist.AccessList, error)
	// EnableRecoveryCodes enables the usage of recovery codes for resetting forgotten passwords
	EnableRecoveryCodes()
	// EnablePlugins enables the hosted plugins runtime
	EnablePlugins()
	// EnableAccessGraph enables the usage of access graph.
	EnableAccessGraph()
	// EnableAccessMonitoring enables the usage of access monitoring.
	EnableAccessMonitoring()
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
	// todo(lxea): DELETE IN 17 [remove env var, leave insecure test mode]
	if GetModules().Features().Cloud ||
		(os.Getenv(teleport.EnvVarAllowNoSecondFactor) != "yes" && !IsInsecureTestMode()) {

		switch r := res.(type) {
		case types.AuthPreference:
			switch r.GetSecondFactor() {
			case constants.SecondFactorOff, constants.SecondFactorOptional:
				return trace.BadParameter("cannot disable two-factor authentication")
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
		Kubernetes:         true,
		DB:                 true,
		App:                true,
		Desktop:            true,
		AutomaticUpgrades:  p.automaticUpgrades,
		Assist:             true,
		JoinActiveSessions: true,
		SupportType:        proto.SupportType_SUPPORT_TYPE_FREE,
	}
}

// SetFeatures sets features queried from Cloud.
// This is a noop since OSS teleport does not support enterprise features
func (p *defaultModules) SetFeatures(f Features) {
}

func (p *defaultModules) IsBoringBinary() bool {
	return native.IsBoringBinary()
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
	accessListGetter AccessListGetter, requestID string,
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
