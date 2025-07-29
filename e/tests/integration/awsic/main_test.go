package awsic

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/integration/helpers"
)

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise, it will run tests as normal.
func TestMain(m *testing.M) {
	// If the leader lock is in contention while restarting the Identity Center
	// integration, by default it will take at least a minute to be acquired.
	// This will almost certainly time out any affected tests. Using this shorter
	// interval brings that interval down into a sensible range for integration tests.
	plugins.LeaderLockRetryInterval = 50 * time.Millisecond

	helpers.TestMainImplementation(m)
}
