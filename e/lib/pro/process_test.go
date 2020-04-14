package pro

import (
	"fmt"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	check "gopkg.in/check.v1"

	"github.com/gravitational/trace"
)

type ProcessSuite struct {
}

var _ = check.Suite(&ProcessSuite{})

func (s *ProcessSuite) TestNotAuthNoLicense(c *check.C) {
	_, err := NewTeleport(&service.Config{
		DataDir: c.MkDir(),
		Proxy: service.ProxyConfig{
			Enabled: true,
		},
		AuthServers: []utils.NetAddr{
			*utils.MustParseAddr("tcp://127.0.0.1:8080"),
		},
	})
	c.Assert(err, check.IsNil)
}

func (s *ProcessSuite) TestAuthNoLicense(c *check.C) {
	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type: "local",
	})
	c.Assert(err, check.IsNil)

	_, err = NewTeleport(&service.Config{
		DataDir: c.MkDir(),
		Auth: service.AuthConfig{
			Enabled: true,
			StorageConfig: backend.Config{
				Type: lite.GetName(),
				Params: backend.Params{
					"path": c.MkDir(),
				},
			},
			ClusterConfig: services.DefaultClusterConfig(),
			StaticTokens:  services.DefaultStaticTokens(),
			NoAudit:       true,
			Preference:    authPreference,
			SSHAddr:       *utils.MustParseAddr("tcp://127.0.0.1:3023"),
		},
		Hostname: "localhost",
		AuthServers: []utils.NetAddr{
			*utils.MustParseAddr("tcp://127.0.0.1:8080"),
		},
	})
	fmt.Printf("%v\n\n", trace.DebugReport(err))
	fixtures.ExpectAccessDenied(c, err)
}
