package integration

import (
	"testing"

	"github.com/gravitational/teleport/integration/helpers"
)

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise, it will run tests as normal.
func TestMain(m *testing.M) {
	helpers.TestMainImplementation(m)
}
