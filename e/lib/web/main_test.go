package web

import (
	"context"
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/cryptosuites/cryptosuitestest"
)

func TestMain(m *testing.M) {

	ctx, cancel := context.WithCancel(context.Background())
	cryptosuitestest.PrecomputeRSAKeys(ctx)
	exitCode := m.Run()
	cancel()
	os.Exit(exitCode)
}
