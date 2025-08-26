package awsic

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/e/lib/aws/identitycenter"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/provisioning"
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

	// The Identity Center event handler processes events in 30s batches in order
	// to discard duplicate events. This is obviously too long for our tests.
	identitycenter.DefaultEventBatchDuration = 50 * time.Millisecond

	// The SCIM provisioner checks whether it needs to update the downstream system
	// every 2 minutes. Tune this down to a test-friendly value
	provisioning.DefaultStateRefreshInterval = 250 * time.Millisecond

	helpers.TestMainImplementation(m)
}
