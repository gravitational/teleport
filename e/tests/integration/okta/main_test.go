package okta

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	libokta "github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/lib/okta/common/connected"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/utils"
	tctlcommon "github.com/gravitational/teleport/tool/tctl/common"
	tshcommon "github.com/gravitational/teleport/tool/tsh/common"
)

const (
	// testBinTCTLTestEnv is the environment variable that will be set to
	// re-execute test as tctl binary.
	testBinTCTLTestEnv = "EXEC_TEST_BIN_TCTL_TEST"
	// testBinTSHTestEnv is the environment variable that will be set to
	// re-execute test as tsh binary.
	testBinTSHTestEnv = "EXEC_TEST_BIN_TSH_TEST"
)

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise, it will run tests as normal.
func TestMain(m *testing.M) {
	if os.Getenv(testBinTCTLTestEnv) != "" {
		if err := os.Setenv(types.HomeEnvVar, os.TempDir()); err != nil {
			utils.FatalError(err)
		}
		tctlcommon.Run(context.Background(), tctlcommon.Commands())
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
	libokta.OktaDefaultTimeBetweenSyncs = time.Second
	libokta.SyncRetryAfterLeadershipFailure = time.Second
	libokta.AccessListSyncFirstDuration = time.Second
	libokta.DefaultAccessListSyncInterval = time.Second
}
