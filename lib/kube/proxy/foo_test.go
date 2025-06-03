package proxy

import (
	"strings"
	"testing"
)

func TestFoo(t *testing.T) {
	f := ForwarderConfig{}
	if strings.Contains(f.ClusterName, "ffoo") {
		t.Fatal("ok")
	}
	f.CheckAndSetDefaults()
}
