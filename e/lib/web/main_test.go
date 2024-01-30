package web

import (
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/auth/native"
)

func TestMain(m *testing.M) {
	native.PrecomputeTestKeys(m)
	os.Exit(m.Run())
}
