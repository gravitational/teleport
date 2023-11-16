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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/e/api/cloud"
	"github.com/gravitational/teleport/e/lib/accessrequest"
	"github.com/gravitational/teleport/e/lib/cloud/feature"
	"github.com/gravitational/teleport/e/lib/hardwarekey"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/modules"
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
	log := logrus.WithField(trace.Component, eModuleComponent)
	p := enterpriseModules{log: log}

	if licenseFile == nil || licenseFile.License == nil {
		modules.SetModules(&p)
		return nil
	}

	features := getLicenseFeatures(licenseFile.License)

	if licenseFile.License.GetFeatureSource() == types.FeatureSourceCloud {
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

		f, err := feature.FetchFromCloud(ctx, client)
		if err != nil {
			p.log.Errorf("failed fetching features from Cloud: %+v", err)
			return trace.Wrap(err)
		}
		p.log.Debugf("successfully fetched features from Cloud: %+v", f)
		f.RecoveryCodes = true
		features = *f
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
	f.IsTrialProduct = p.features.IsTrialProduct
	f.AccessGraph = p.features.AccessGraph

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

// BuildType returns build type (OSS or Enterprise)
func (p *enterpriseModules) BuildType() string {
	return modules.BuildEnterprise
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
func (p *enterpriseModules) AttestHardwareKey(ctx context.Context, serverI interface{}, requiredKeyPolicy keys.PrivateKeyPolicy, att *keys.AttestationStatement, pub crypto.PublicKey, sessionTTL time.Duration) (keys.PrivateKeyPolicy, error) {
	// serverI is passed as a plain interface{} to make it more cryptic,
	// and therefore difficult for OSS users to implement themselves 😈
	server, ok := serverI.(hardwarekey.AttestationServer)
	if !ok {
		return "", trace.BadParameter("Received unexpected server interface of type %T", serverI)
	}
	return hardwarekey.AttestHardwareKey(ctx, server, requiredKeyPolicy, att, pub, sessionTTL)
}

func (p *enterpriseModules) GenerateAccessRequestPromotions(ctx context.Context, accessListGetter modules.AccessResourcesGetter, accessRequest types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	return accessrequest.GenerateAccessRequestPromotions(ctx, accessListGetter, accessRequest)
}

func getLicenseFeatures(license types.License) modules.Features {
	// All features are always enabled in Teleport Cloud since it does a
	// per-resource usage reporting. Also, for backward compatibility so
	// we don't need to reissue licenses every time we add a new feature.
	return modules.Features{
		Kubernetes:              license.GetCloud().Value() || license.GetSupportsKubernetes().Value(),
		App:                     license.GetCloud().Value() || license.GetSupportsApplicationAccess().Value(),
		DB:                      license.GetCloud().Value() || license.GetSupportsDatabaseAccess().Value(),
		Desktop:                 license.GetCloud().Value() || license.GetSupportsDesktopAccess().Value(),
		Cloud:                   license.GetCloud().Value(),
		OIDC:                    true,
		SAML:                    true,
		AccessControls:          true,
		AdvancedAccessWorkflows: true,
		HSM:                     true,
		RecoveryCodes:           license.GetCloud().Value(),
		IsUsageBasedBilling:     false, // usage-based subscriptions don't use license as source of features
		FeatureHiding:           license.GetSupportsFeatureHiding().Value(),
		CustomTheme:             license.GetCustomTheme(),
		// Assist is disabled by default on Cloud.
		// In case of the Team plan, this gets overridden to `true` by the dynamic features from Sales Center
		Assist: !license.GetCloud().Value(),
		// Device trust is enabled and unlimited, by default, for Enterprise/Cloud.
		// Team accounts limitations are handled by [feature.FetchFromCloud].
		DeviceTrust: modules.DeviceTrustFeature{
			Enabled: true,
		},
		IsTrialProduct: license.GetTrial().Value(),
	}
}
