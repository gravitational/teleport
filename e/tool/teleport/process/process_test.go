package process

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestProxyWithoutLicense(t *testing.T) {
	config := &service.Config{
		DataDir: t.TempDir(),
		Proxy: service.ProxyConfig{
			Enabled: true,
		},
		AuthServers: []utils.NetAddr{
			*utils.MustParseAddr("tcp://127.0.0.1:8080"),
		},
		Log: utils.WrapLogger(logrus.WithField("test", t.Name())),
	}

	_, err := NewTeleport(config)
	require.Nil(t, err)
}

func TestMissingLicenseError(t *testing.T) {
	authPreference, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type: "local",
	})
	require.Nil(t, err)

	config := &service.Config{
		DataDir: t.TempDir(),
		Auth: service.AuthConfig{
			Enabled: true,
			StorageConfig: backend.Config{
				Type: lite.GetName(),
				Params: backend.Params{
					"path": t.TempDir(),
				},
			},
			StaticTokens: types.DefaultStaticTokens(),
			NoAudit:      true,
			Preference:   authPreference,
			SSHAddr:      *utils.MustParseAddr("tcp://127.0.0.1:0"),
		},
		Hostname: "localhost",
		AuthServers: []utils.NetAddr{
			*utils.MustParseAddr("tcp://127.0.0.1:8080"),
		},
		Log: utils.WrapLogger(logrus.WithField("test", t.Name())),
	}

	_, err = NewTeleport(config)
	require.True(t, trace.IsAccessDenied(err))
}
