package okta

import (
	"os"
	"testing"
	"time"

	libokta "github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/lib/okta/common/connected"
	"github.com/gravitational/teleport/e/tests/common/tctl"
	"github.com/gravitational/teleport/integration/helpers"
	tshcommon "github.com/gravitational/teleport/tool/tsh/common"
)

const (

	// testBinTSHTestEnv is the environment variable that will be set to
	// re-execute test as tsh binary.
	testBinTSHTestEnv = "EXEC_TEST_BIN_TSH_TEST"
)

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise, it will run tests as normal.
func TestMain(m *testing.M) {
	if runTctl, isTctl := tctl.IsReExec(); isTctl {
		runTctl()
		return
	}
	if os.Getenv(testBinTSHTestEnv) != "" {
		tshcommon.Main()
		return
	}

	alignOktaSyncTimes()
	helpers.TestMainImplementation(m)
}

// TestOktaSyncTimes will align the Okta sync times to ensure that the tests
// run quicker.
func alignOktaSyncTimes() {
	connected.DefaultOktaConnectedCacheTTL = time.Millisecond * 100
	libokta.SyncRetryAfterLeadershipFailure = time.Second
	libokta.AccessRequestCheckInventoryInterval = time.Second
}
