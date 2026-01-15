package web

import (
	"context"
	"os"
	"testing"

	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/cryptosuites/cryptosuitestest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestMain(m *testing.M) {
	modules.SetModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.Policy: {Enabled: true},
			},
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	cryptosuitestest.PrecomputeRSAKeys(ctx)
	exitCode := m.Run()
	cancel()
	os.Exit(exitCode)
}
