package identitycenter

import (
	"os"
	"testing"

	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func TestMain(m *testing.M) {
	// Almost every test in this package requires the Access List feature, so
	// we enable it once here, as opposed to using [modulestest.SetTestModules()] in
	// every test, which would also force each test to run in series.
	modules.SetModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {Enabled: true},
			},
		},
	})

	os.Exit(m.Run())
}
