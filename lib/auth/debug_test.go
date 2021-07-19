package auth

import (
	"runtime"
	"testing"
)

func TestDebug(t *testing.T) {
	t.Fatalf("--> GOMAXPROCS: %v.\n", runtime.GOMAXPROCS(0))
}
