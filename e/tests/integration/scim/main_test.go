package scim

import (
	"context"
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/cryptosuites/cryptosuitestest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	modules.SetInsecureTestMode(true)

	ctx, cancel := context.WithCancel(context.Background())
	cryptosuitestest.PrecomputeRSAKeys(ctx)
	exitCode := m.Run()
	cancel()
	os.Exit(exitCode)
}
