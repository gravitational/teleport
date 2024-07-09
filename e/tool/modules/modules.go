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

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/e/api/cloud"
	"github.com/gravitational/teleport/e/lib/accessrequest"
	"github.com/gravitational/teleport/e/lib/cloud/feature"
	"github.com/gravitational/teleport/e/lib/hardwarekey"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
)

// eModuleComponent is the name of the component used for logging
const eModuleComponent = "enterprise/modules"

// cloudFeatureRequestTimeout is the timeout of requests to Teleport Cloud
// when fetching features
var cloudFeatureRequestTimeout = time.Second * 10

func init() {
	// Set the modules to Enterprise but with no license information.
	modules.SetModules(&enterpriseModules{})
}

// SetModules installs modules that provide custom behavior for the
// enterprise compared to the open-source version
func SetModules(licenseFile *licensefile.LicenseFile) error {
	p := enterpriseModules{log: logrus.WithField(teleport.ComponentKey, eModuleComponent)}
	if licenseFile == nil || licenseFile.License == nil {
		modules.SetModules(&p)
		return nil
	}

	p.licenseExpiry = licenseFile.License.Expiry()
	features := getSelfHostedLicenseFeatures(licenseFile.License)

	// Fetch supported features from salescenter "subscriptions" db table for
	// cloud based subscriptions:
	//   - Team
	//   - Enterprise Usage Based Cloud
	//   - Legacy Enterprise Teleport Cloud (non-usage based but hosted by Teleport)
	if licenseFile.License.GetCloud() {
		p.log.Debug("fetching features from Cloud")
		tlsConfig, err := liblicense.MakeTLSConfig(*licenseFile.KeyPair)
		if err != nil {
			return trace.Wrap(err)
		}

		client, err := cloud.NewClientFromTLSConfig(tlsConfig)
		if err != nil {
			p.log.Errorf("failed creating cloud client to fetch features: %+v", err)
			return trace.Wrap(err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), cloudFeatureRequestTimeout)
		defer cancel()

		f, err := feature.GetCloudFeatures(ctx, client)
		if err != nil {
			p.log.Errorf("failed fetching features from Cloud: %+v", err)
			return trace.Wrap(err)
		}
		p.log.Debugf("successfully fetched features from Cloud: %+v", f)
		f.RecoveryCodes = true
		features = *f
	}

	if cloud.IsCloudEnv() {
		features.RecoveryCodes = true
	}

	p.features = features

	modules.SetModules(&p)
	return nil
}

// enterpriseModules implements pluggable enterprise teleport logic
type enterpriseModules struct {
	mu sync.RWMutex
	// features is the feature set of the cluster
	features          modules.Features
	log               *logrus.Entry
	licenseExpiry     time.Time
	automaticUpgrades bool
	loadDynamicValues sync.Once
}

// Features returns supported features
func (p *enterpriseModules) Features() modules.Features {
	p.mu.RLock()
	defer p.mu.RUnlock()

	features := p.features

	p.loadDynamicValues.Do(func() {
		p.automaticUpgrades = automaticupgrades.IsEnabled()
	})

	features.AutomaticUpgrades = p.automaticUpgrades
	return features
}

// SetFeatures sets the module's features for cloud clusters.
// The values of RecoveryCodes and Plugins will not be updated. Use
// EnableRecoveryCodes or EnablePlugins to update these fields.
func (p *enterpriseModules) SetFeatures(f modules.Features) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// since these fields are directly set based on config features and shouldn't be set automatically
	f.RecoveryCodes = p.features.RecoveryCodes
	f.Plugins = p.features.Plugins
	f.AccessGraph = p.features.AccessGraph
	f.AccessMonitoringConfigured = p.features.AccessMonitoringConfigured

	p.features = f
}

// EnableRecoveryCodes enables the usage of recovery codes for resetting forgotten passwords
func (p *enterpriseModules) EnableRecoveryCodes() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.features.RecoveryCodes = true
}

// EnablePlugins enables the hosted plugins runtime.
func (p *enterpriseModules) EnablePlugins() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.features.Plugins = true
}

// EnableAccessGraph enables the usage of access graph.
func (p *enterpriseModules) EnableAccessGraph() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.features.AccessGraph = true
}

// EnableAccessMonitoring enables the usage of access monitoring.
func (p *enterpriseModules) EnableAccessMonitoring() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.features.AccessMonitoringConfigured = true
}

// BuildType returns build type (OSS or Enterprise)
func (p *enterpriseModules) BuildType() string {
	return modules.BuildEnterprise
}

// IsEnterpriseBuild returns true for [enterpriseModules].
func (p *enterpriseModules) IsEnterpriseBuild() bool {
	return true
}

// IsOSSBuild returns false for [enterpriseModules].
func (p *enterpriseModules) IsOSSBuild() bool {
	return false
}

// LicenseExpiry returns the license expiry time.
func (p *enterpriseModules) LicenseExpiry() time.Time {
	return p.licenseExpiry
}

// PrintVersion prints the Teleport version. For enterprise it includes
// "Enterprise" in the output.
func (p *enterpriseModules) PrintVersion() {
	fmt.Printf("Teleport Enterprise v%s git:%s %s\n", teleport.Version, teleport.Gitref, runtime.Version())
}

// IsBoringBinary checks if the binary was compiled with BoringCrypto.
func (p *enterpriseModules) IsBoringBinary() bool {
	// Check the package name for one of the boring primitives, if the package
	// path is from BoringCrypto, we know this binary was compiled against the
	// dev.boringcrypto branch of Go.
	hash := sha256.New()
	return reflect.TypeOf(hash).Elem().PkgPath() == "crypto/internal/boring"
}

// AttestHardwareKey attests a hardware key, either with the given statement or
// previously stored attestation data matching the given public key.
func (p *enterpriseModules) AttestHardwareKey(ctx context.Context, serverI interface{}, att *keys.AttestationStatement, pub crypto.PublicKey, sessionTTL time.Duration) (*keys.AttestationData, error) {
	// serverI is passed as a plain interface{} to make it more cryptic,
	// and therefore difficult for OSS users to implement themselves 😈
	server, ok := serverI.(hardwarekey.AttestationServer)
	if !ok {
		return nil, trace.BadParameter("Received unexpected server interface of type %T", serverI)
	}
	return hardwarekey.AttestHardwareKey(ctx, server, att, pub, sessionTTL)
}

func (p *enterpriseModules) GenerateAccessRequestPromotions(ctx context.Context, accessListGetter modules.AccessResourcesGetter, accessRequest types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	return accessrequest.GenerateAccessRequestPromotions(ctx, accessListGetter, accessRequest)
}

func (p *enterpriseModules) GetSuggestedAccessLists(ctx context.Context, identity *tlsca.Identity, clt modules.AccessListSuggestionClient,
	accessListGetter modules.AccessListGetter, requestID string,
) ([]*accesslist.AccessList, error) {
	return accessrequest.GetSuggestedAccessLists(ctx, identity, clt, accessListGetter, requestID)
}

// getSelfHostedLicenseFeatures is only used to read features for `self-hosted` licenses.
// For cloud subscriptions, the features are read from `GetCloudFeatures`.
func getSelfHostedLicenseFeatures(license types.License) modules.Features {
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
		f.Entitlements[entitlements.AccessRequests] = modules.EntitlementInfo{Enabled: true}
		f.Entitlements[entitlements.DeviceTrust] = modules.EntitlementInfo{Enabled: true}
		f.Entitlements[entitlements.AccessMonitoring] = modules.EntitlementInfo{Enabled: true}
		f.Entitlements[entitlements.AccessLists] = modules.EntitlementInfo{Enabled: true}
	}

	return f
}
