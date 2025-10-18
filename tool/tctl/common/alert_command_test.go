package common

import (
	"bytes"
	"context"
	"testing"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/tool/teleport/testenv"
	"github.com/stretchr/testify/require"
)

// TestAlertAcks tests formatting (json, yaml) for alerts ack add, alerts ack list.
func TestAlertAcks(t *testing.T) {
	process, err := testenv.NewTeleportProcess(t.TempDir(), testenv.WithLogger(logtest.NewLogger()))
	require.NoError(t, err)
	ctx := context.Background()
	client, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	alert1, err := types.NewClusterAlert("alert-1", "some msg")
	require.NoError(t, err)
	alert2, err := types.NewClusterAlert("alert-2", "some msg")
	require.NoError(t, err)

	err = client.UpsertClusterAlert(ctx, alert1)
	require.NoError(t, err)
	err = client.UpsertClusterAlert(ctx, alert2)
	require.NoError(t, err)

	// test all output formats of "alerts ack" (add alert acknowledgement)
	buf, err := runAlertCommand(t, client, []string{"ack", "alert-1", "--reason=none", "--format", teleport.JSON})
	require.NoError(t, err)
	out := mustDecodeJSON[types.AlertAcknowledgement](t, buf)
	require.Equal(t, out.AlertID, "alert-1")

	buf, err = runAlertCommand(t, client, []string{"ack", "alert-2", "--reason=none", "--format", teleport.YAML})
	require.NoError(t, err)

	// transcode yaml to json, since AlertAcknowledgement does not contain yaml tags
	// for safe decoding
	yamlBuf := mustTranscodeYAMLToJSON(t, buf)
	out = mustDecodeJSON[types.AlertAcknowledgement](t, bytes.NewReader(yamlBuf))
	require.Equal(t, out.AlertID, "alert-2")

	// test all output formats of "alerts ack ls"
	buf, err = runAlertCommand(t, client, []string{"ack", "ls", "--format", teleport.JSON})
	require.NoError(t, err)

	jsonOut := mustDecodeJSON[[]types.AlertAcknowledgement](t, buf)
	require.Len(t, jsonOut, 2)

	buf, err = runAlertCommand(t, client, []string{"ack", "ls", "--format", teleport.YAML})
	require.NoError(t, err)

	var yamlOut []types.AlertAcknowledgement
	yamlBuf = mustTranscodeYAMLDocsToJSON(t, buf)
	yamlOut = mustDecodeJSON[[]types.AlertAcknowledgement](t, bytes.NewReader(yamlBuf))
	require.Len(t, yamlOut, 2)

	require.Equal(t, jsonOut, yamlOut)
}
