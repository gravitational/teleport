package web

import (
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/cryptosuites"
)

func TestMain(m *testing.M) {
	cryptosuites.PrecomputeRSATestKeys(m)
	os.Exit(m.Run())
}
