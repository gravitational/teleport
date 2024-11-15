package plugins

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	ictestenv "github.com/gravitational/teleport/e/lib/aws/identitycenter/test"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestStatusSink(t *testing.T) {
	const pluginName = "foo"

	ctx := context.Background()
	mem, err := memory.New(memory.Config{
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	backendService := local.NewPluginsService(mem)
	initialPlugin := createSlackPlugin(t, pluginName).(*types.PluginV1)
	require.NoError(t, backendService.CreatePlugin(ctx, initialPlugin))
	statusSink := newStatusSink(backendService, pluginName, string(initialPlugin.GetType()))

	newStatus := &types.PluginStatusV1{
		Code: types.PluginStatusCode_UNAUTHORIZED,
	}
	err = statusSink.Emit(ctx, newStatus)
	require.NoError(t, err)

	gotPlugin, err := backendService.GetPlugin(ctx, pluginName, true)
	require.NoError(t, err)
	require.Equal(t, newStatus, gotPlugin.GetStatus())

	// Other fields of the plugin resource should remain untouched
	require.Empty(t, cmp.Diff(initialPlugin.Metadata, gotPlugin.GetMetadata(), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	require.Equal(t, initialPlugin.Spec, gotPlugin.(*types.PluginV1).Spec)
	require.Equal(t, initialPlugin.Credentials, gotPlugin.GetCredentials())
}

func TestIdentityCenterStatusSink(t *testing.T) {
	const pluginName = types.PluginTypeAWSIdentityCenter

	ctx := context.Background()
	mem, err := memory.New(memory.Config{
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	backendService := local.NewPluginsService(mem)
	createPluginReq := ictestenv.NewPluginV1CreateRequest("test-oidc", "test-saml")
	initialPlugin := types.NewPluginV1(createPluginReq.GetPlugin().GetMetadata(), createPluginReq.GetPlugin().Spec, nil)
	require.NoError(t, backendService.CreatePlugin(ctx, initialPlugin))
	statusSink := newStatusSink(backendService, pluginName, string(initialPlugin.GetType()))

	newStatus := &types.PluginStatusV1{
		Code: types.PluginStatusCode_RUNNING,
		Details: &types.PluginStatusV1_AwsIc{
			AwsIc: &types.PluginAWSICStatusV1{
				GroupImportStatus: &types.AWSICGroupImportStatus{
					StatusCode: types.AWSICGroupImportStatusCode_FAILED,
				},
			},
		},
	}
	err = statusSink.Emit(ctx, newStatus)
	require.NoError(t, err)
	pluginFromBackend, err := backendService.GetPlugin(ctx, pluginName, true)
	require.NoError(t, err)
	require.Equal(t, newStatus, pluginFromBackend.GetStatus())
	// Other fields of the plugin resource should remain untouched
	require.Empty(t, cmp.Diff(initialPlugin.Metadata, pluginFromBackend.GetMetadata(), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
	require.Equal(t, initialPlugin.Spec, pluginFromBackend.(*types.PluginV1).Spec)
	require.Equal(t, initialPlugin.Credentials, pluginFromBackend.GetCredentials())

	// emit error status
	errorStatus := &types.PluginStatusV1{
		Code: types.PluginStatusCode_OTHER_ERROR,
	}
	err = statusSink.Emit(ctx, errorStatus)
	require.NoError(t, err)
	// existing detailed status field should be preserved.
	pluginFromBackend, err = backendService.GetPlugin(ctx, pluginName, true)
	require.NoError(t, err)
	require.Equal(t, errorStatus.GetCode(), pluginFromBackend.GetStatus().GetCode())
	require.Equal(t, newStatus.GetAwsIc(), pluginFromBackend.GetStatus().GetAwsIc())

	// re-emit status group import status field
	successStatus := &types.PluginStatusV1{
		Code: types.PluginStatusCode_RUNNING,
		Details: &types.PluginStatusV1_AwsIc{
			AwsIc: &types.PluginAWSICStatusV1{
				GroupImportStatus: &types.AWSICGroupImportStatus{
					StatusCode: types.AWSICGroupImportStatusCode_DONE,
				},
			},
		},
	}
	err = statusSink.Emit(ctx, successStatus)
	require.NoError(t, err)
	pluginFromBackend, err = backendService.GetPlugin(ctx, pluginName, true)
	require.NoError(t, err)
	require.Equal(t, successStatus.GetCode(), pluginFromBackend.GetStatus().GetCode())
	require.Equal(t, successStatus.GetAwsIc(), pluginFromBackend.GetStatus().GetAwsIc())
}
