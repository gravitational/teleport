package modules

import (
	"context"
	"crypto"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/e/lib/accessrequest"
	"github.com/gravitational/teleport/e/lib/cloud/feature"
	ehardwarekey "github.com/gravitational/teleport/e/lib/hardwarekey"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
)

// EnterpriseModules implements pluggable enterprise teleport logic
type EnterpriseModules struct {
	licenseExpiry     time.Time
	automaticUpgrades bool
	accessMonitoring  bool
	accessGraph       bool
	recoveryCodes     bool
	plugins           bool

	mu sync.RWMutex
	// features is the feature set of the cluster
	features modules.Features
}

// EnterpriseModulesConfig contains dependencies required to create [EnterpriseModules].
type EnterpriseModulesConfig struct {
	License                  *licensefile.LicenseFile
	Features                 modules.Features
	HostedPluginsEnabled     bool
	Cloud                    bool
	AutomaticUpgradesEnabled bool
}

// NewEnterpriseModules creates an [EnterpriseModules] from the provided dependencies.
func NewEnterpriseModules(cfg EnterpriseModulesConfig) *EnterpriseModules {
	p := &EnterpriseModules{
		features:          cfg.Features,
		recoveryCodes:     cfg.Cloud,
		plugins:           cfg.HostedPluginsEnabled,
		automaticUpgrades: cfg.AutomaticUpgradesEnabled,
		accessMonitoring:  cfg.Features.GetEntitlement(entitlements.AccessMonitoring).Enabled,
		accessGraph:       cfg.Features.GetEntitlement(entitlements.Policy).Enabled,
	}

	if cfg.License != nil && cfg.License.License != nil {
		p.licenseExpiry = cfg.License.License.Expiry()
	}

	return p
}

// UpdateModules sets the module's features and license information. This is similar to
// SetFeatures, but should be preferred when updating features and license information.
func (p *EnterpriseModules) UpdateModules(licenseFile *licensefile.LicenseFile, features modules.Features) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if licenseFile != nil && licenseFile.License != nil {
		p.licenseExpiry = licenseFile.License.Expiry()
	}

	p.features = features
}

// Features returns supported features
func (p *EnterpriseModules) Features() modules.Features {
	p.mu.RLock()
	defer p.mu.RUnlock()

	features := p.features

	features.AutomaticUpgrades = p.automaticUpgrades
	// RecoveryCodes are enabled during startup when running on Cloud
	features.RecoveryCodes = p.recoveryCodes
	// Plugins are always enabled based on auth file config
	features.Plugins = p.plugins
	// AccessGraph is enabled at startup when the Entitlements Policy is enabled
	features.AccessGraph = p.accessGraph
	// AccessMonitoringConfigured is enabled at startup when
	// the entitlement AccessMonitoring is enabled
	features.AccessMonitoringConfigured = p.accessMonitoring
	return features
}

// SetFeatures sets the module's features for cloud clusters.
// The values of RecoveryCodes and Plugins will not be updated. Use
// EnableRecoveryCodes or EnablePlugins to update these fields.
func (p *EnterpriseModules) SetFeatures(f modules.Features) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.features = f
}

// EnableRecoveryCodes enables the usage of recovery codes for resetting forgotten passwords
// TODO(tross) remove once modules.Modules interface is updated.
func (p *EnterpriseModules) EnableRecoveryCodes() {}

// EnablePlugins enables the hosted plugins runtime.
// TODO(tross) remove once modules.Modules interface is updated.
func (p *EnterpriseModules) EnablePlugins() {}

// EnableAccessGraph enables the usage of access graph.
func (p *EnterpriseModules) EnableAccessGraph() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.accessGraph = true
}

// EnableAccessMonitoring enables the usage of access monitoring.
func (p *EnterpriseModules) EnableAccessMonitoring() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.accessMonitoring = true
}

// BuildType returns build type (OSS or Enterprise)
func (p *EnterpriseModules) BuildType() string {
	return modules.BuildEnterprise
}

// IsEnterpriseBuild returns true for [enterpriseModules].
func (p *EnterpriseModules) IsEnterpriseBuild() bool {
	return true
}

// IsOSSBuild returns false for [enterpriseModules].
func (p *EnterpriseModules) IsOSSBuild() bool {
	return false
}

// LicenseExpiry returns the expiry date of the enterprise license, if applicable.
func (p *EnterpriseModules) LicenseExpiry() time.Time {
	return p.licenseExpiry
}

// PrintVersion prints the Teleport version. For enterprise it includes
// "Enterprise" in the output.
func (p *EnterpriseModules) PrintVersion() {
	fmt.Printf("Teleport Enterprise v%s git:%s %s\n", teleport.Version, teleport.Gitref, runtime.Version())
}

// IsFIPSBuild checks if the binary was compiled in FIPS140 mode.
func (p *EnterpriseModules) IsFIPSBuild() bool {
	return modules.IsFIPSBuild()
}

// AttestHardwareKey attests a hardware key, either with the given statement or
// previously stored attestation data matching the given public key.
func (p *EnterpriseModules) AttestHardwareKey(ctx context.Context, serverI any, att *hardwarekey.AttestationStatement, pub crypto.PublicKey, sessionTTL time.Duration) (*keys.AttestationData, error) {
	// serverI is passed as a plain interface{} to make it more cryptic,
	// and therefore difficult for OSS users to implement themselves 😈
	server, ok := serverI.(ehardwarekey.AttestationServer)
	if !ok {
		return nil, trace.BadParameter("Received unexpected server interface of type %T", serverI)
	}
	return ehardwarekey.AttestHardwareKey(ctx, server, att, pub, sessionTTL)
}

func (p *EnterpriseModules) GenerateLongTermResourceGrouping(ctx context.Context, clt modules.AccessResourcesGetter, req types.AccessRequest) (*types.LongTermResourceGrouping, error) {
	return accessrequest.GenerateLongTermResourceGrouping(ctx, clt, req)
}

func (p *EnterpriseModules) GenerateAccessRequestPromotions(ctx context.Context, accessListGetter modules.AccessResourcesGetter, accessRequest types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	return accessrequest.GenerateAccessRequestPromotions(ctx, accessListGetter, accessRequest)
}

// GenerateAccessRequestSuggestedReviewers returns a list of suggested reviewers for a given access request.
func (p *EnterpriseModules) GenerateAccessRequestSuggestedReviewers(ctx context.Context, accessListGetter modules.AccessResourcesGetter, accessRequest types.AccessRequest) ([]string, error) {
	return accessrequest.GenerateAccessRequestSuggestedReviewers(ctx, accessListGetter, accessRequest)
}

func (p *EnterpriseModules) GetSuggestedAccessLists(ctx context.Context, identity *tlsca.Identity, clt modules.AccessListSuggestionClient,
	accessListGetter modules.AccessListAndMembersGetter, requestID string,
) ([]*accesslist.AccessList, error) {
	return accessrequest.GetSuggestedAccessLists(ctx, identity, clt, accessListGetter, requestID)
}

// GetSelfHostedLicenseFeatures is only used to read features for `self-hosted` licenses.
// For cloud subscriptions, the features are read from `GetCloudFeatures`.
func GetSelfHostedLicenseFeatures(license types.License) modules.Features {
	// Set Legacy Values first; backwards compatibility with older licenses
	f := setLegacyLogic(license)

	// Set Entitlement values last; override features if present; use Modern license values
	if license.GetEntitlements() != nil && len(license.GetEntitlements()) > 0 {
		e := license.GetEntitlements()
		f.Entitlements = feature.GetLicenseEntitlements(e)
	}

	return f
}

func setLegacyLogic(license types.License) modules.Features {
	f := modules.Features{
		// From License
		CustomTheme:         license.GetCustomTheme(),
		RecoveryCodes:       license.GetCloud().Value(),
		IsUsageBasedBilling: license.GetUsageBasedBilling().Value(),
		Cloud:               license.GetCloud().Value(),
		// Hard-coded
		AccessControls:          true,
		AdvancedAccessWorkflows: true,
		IsStripeManaged:         false,
		Questionnaire:           false,
		SupportType:             proto.SupportType_SUPPORT_TYPE_PREMIUM,
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			// from License
			entitlements.App:           {Enabled: license.GetSupportsApplicationAccess().Value()},
			entitlements.DB:            {Enabled: license.GetSupportsDatabaseAccess().Value()},
			entitlements.Desktop:       {Enabled: license.GetSupportsDesktopAccess().Value()},
			entitlements.FeatureHiding: {Enabled: license.GetSupportsFeatureHiding().Value()},
			entitlements.Identity:      {Enabled: license.GetSupportsIdentityGovernanceSecurity().Value()},
			entitlements.K8s:           {Enabled: license.GetSupportsKubernetes().Value()},
			entitlements.Policy:        {Enabled: license.GetSupportsPolicy().Value()},
			// Hard-coded
			entitlements.ExternalAuditStorage:   {Enabled: false},
			entitlements.HSM:                    {Enabled: true},
			entitlements.JoinActiveSessions:     {Enabled: true},
			entitlements.MobileDeviceManagement: {Enabled: true},
			entitlements.OIDC:                   {Enabled: true},
			entitlements.SAML:                   {Enabled: true},

			// The following features are not applicable to self-hosted customers
			entitlements.CloudAuditLogRetention: {Enabled: false},
			entitlements.UpsellAlert:            {Enabled: false},
			entitlements.UsageReporting:         {Enabled: false},
			entitlements.OktaSCIM:               {Enabled: false},
			entitlements.OktaUserSync:           {Enabled: false},
			entitlements.SessionLocks:           {Enabled: false},

			// Default all customers to Identity entitlement previews
			entitlements.AccessRequests:   {Enabled: true, Limit: 5},
			entitlements.DeviceTrust:      {Enabled: true, Limit: 5},
			entitlements.AccessMonitoring: {Enabled: true, Limit: 30},
			entitlements.AccessLists:      {Enabled: true, Limit: 1},
		},
	}
	// There is only two types of `on-prem` licenses:
	//  1. enterprise usage based (EUB)
	//  2. non usage based: refers to legacy license before EUB product.
	if f.IsUsageBasedBilling {
		f.ProductType = modules.ProductTypeEUB
	}

	// Enable Access Graph, Activity Center and Session Summaries if Policy is enabled,
	// as these features depend on Policy; this is for backwards compatibility with
	// older licenses that may not have these entitlements explicitly set,
	// but should have them enabled if they have Policy enabled.
	if license.GetSupportsPolicy().Value() {
		f.Entitlements[entitlements.AccessGraph] = modules.EntitlementInfo{Enabled: true}
		f.Entitlements[entitlements.ActivityCenter] = modules.EntitlementInfo{Enabled: true}
		f.Entitlements[entitlements.SessionSummaries] = modules.EntitlementInfo{Enabled: true}
	}

	// Set Identity Entitlements second; override legacy license with hard coded previews; backwards compatibility with older licenses
	// Override legacy license values
	legacyLicense := !f.IsUsageBasedBilling
	if legacyLicense && !f.Entitlements[entitlements.Identity].Enabled {
		// Legacy licenses (non-usage based) without Identity will continue to have unlimited support
		// for feature AccessRequests & DeviceTrust.
		f.Entitlements[entitlements.AccessRequests] = modules.EntitlementInfo{Enabled: true}
		f.Entitlements[entitlements.DeviceTrust] = modules.EntitlementInfo{Enabled: true}
	}

	// Override if Identity is enabled (unlimited & enabled identity entitlements)
	if f.Entitlements[entitlements.Identity].Enabled {
		f.Entitlements[entitlements.AccessLists] = modules.EntitlementInfo{Enabled: true}
		f.Entitlements[entitlements.AccessMonitoring] = modules.EntitlementInfo{Enabled: true}
		f.Entitlements[entitlements.AccessRequests] = modules.EntitlementInfo{Enabled: true}
		f.Entitlements[entitlements.DeviceTrust] = modules.EntitlementInfo{Enabled: true}
		f.Entitlements[entitlements.OktaSCIM] = modules.EntitlementInfo{Enabled: true}
		f.Entitlements[entitlements.OktaUserSync] = modules.EntitlementInfo{Enabled: true}
		f.Entitlements[entitlements.SessionLocks] = modules.EntitlementInfo{Enabled: true}
	}

	return f
}
