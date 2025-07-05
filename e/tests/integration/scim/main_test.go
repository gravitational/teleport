package scim

import (
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	modules.SetInsecureTestMode(true)
	cryptosuites.PrecomputeRSATestKeys(m)
	os.Exit(m.Run())
}
