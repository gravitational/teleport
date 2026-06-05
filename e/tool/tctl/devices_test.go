package main_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	authe "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestDevicesFormat(t *testing.T) {
	t.Parallel()
	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			testModules := &modulestest.Modules{
				TestBuildType: modules.BuildEnterprise,
				TestFeatures: modules.Features{
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.DeviceTrust: {Enabled: true},
					},
				},
			}

			cfg.PluginRegistry = plugin.NewRegistry()
			cfg.Modules = testModules
			authPlugin, err := authe.NewPlugin(authe.Config{
				License: authe.ValidLicense{},
				Modules: testModules,
			})
			require.NoError(t, err)
			err = cfg.PluginRegistry.Add(authPlugin)
			require.NoError(t, err)
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})

	client, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	// test all output formats of "devices add"
	buf, err := runDevicesCommand(t, client, []string{"add", "--os=macos", "--asset-tag=dummy1", "--format", teleport.JSON})
	require.NoError(t, err)
	out := mustDecodeJSON[*devicepb.Device](t, buf)
	require.Equal(t, "dummy1", out.GetAssetTag())
	require.Equal(t, devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED, out.GetEnrollStatus())

	buf, err = runDevicesCommand(t, client, []string{"add", "--os=macos", "--asset-tag=dummy2", "--format", teleport.YAML})
	require.NoError(t, err)
	// safely transcode yaml to json into (devicepb.Device) struct
	yamlBuf := mustTranscodeYAMLToJSON(t, buf)
	out = mustDecodeJSON[*devicepb.Device](t, bytes.NewReader(yamlBuf))

	require.Equal(t, "dummy2", out.GetAssetTag())
	require.Equal(t, devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED, out.GetEnrollStatus())

	// test all output formats of "devices ls"
	buf, err = runDevicesCommand(t, client, []string{"ls", "--format", teleport.JSON})
	require.NoError(t, err)
	jsonOut := mustDecodeJSON[[]*devicepb.Device](t, buf)
	require.Len(t, jsonOut, 2)

	buf, err = runDevicesCommand(t, client, []string{"ls", "--format", teleport.YAML})
	require.NoError(t, err)
	var yamlOut []*devicepb.Device
	yamlBuf = mustTranscodeYAMLDocsToJSON(t, buf)
	yamlOut = mustDecodeJSON[[]*devicepb.Device](t, bytes.NewReader(yamlBuf))
	require.Len(t, yamlOut, 2)
	require.Equal(t, jsonOut, yamlOut)
}
