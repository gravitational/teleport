package modules

import (
	"context"
	"crypto"
	"crypto/sha256"
	"fmt"
	"reflect"
	"runtime"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/e/lib/hardwarekey"
	"github.com/gravitational/teleport/lib/modules"
)

func init() {
	// Set the modules to Enterprise but with no license information.
	modules.SetModules(&enterpriseModules{})
}

// SetModules installs modules that provide custom behavior for the
// enterprise compared to the open-source version
func SetModules(license types.License) {
	modules.SetModules(&enterpriseModules{license: license})
}

// enterpriseModules implements pluggable enterprise teleport logic
type enterpriseModules struct {
	license types.License
}

// Features returns supported features
func (p *enterpriseModules) Features() modules.Features {
	if p.license == nil {
		return modules.Features{}
	}
	// All features are always enabled in Teleport Cloud since it does a
	// per-resource usage reporting. Also, for backward compatibility so
	// we don't need to reissue licenses every time we add a new feature.
	return modules.Features{
		Kubernetes:              p.license.GetCloud().Value() || p.license.GetSupportsKubernetes().Value(),
		App:                     p.license.GetCloud().Value() || p.license.GetSupportsApplicationAccess().Value(),
		DB:                      p.license.GetCloud().Value() || p.license.GetSupportsDatabaseAccess().Value(),
		Desktop:                 p.license.GetCloud().Value() || p.license.GetSupportsDesktopAccess().Value(),
		Cloud:                   p.license.GetCloud().Value(),
		OIDC:                    true,
		SAML:                    true,
		AccessControls:          true,
		AdvancedAccessWorkflows: true,
		HSM:                     true,
	}
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
