/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestAlertAcks tests formatting (json, yaml) for alerts ack add, alerts ack list.
func TestAlertAcks(t *testing.T) {
	process, err := testenv.NewTeleportProcess(t.TempDir(), testenv.WithLogger(logtest.NewLogger()))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})
	client, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	// test all output formats of "alerts ack" (add alert acknowledgement)
	buf, err := runAlertCommand(t, client, []string{"ack", "alert-1", "--reason=none", "--format", teleport.JSON})
	require.NoError(t, err)
	out := mustDecodeJSON[types.AlertAcknowledgement](t, buf)
	require.Equal(t, "alert-1", out.AlertID)

	buf, err = runAlertCommand(t, client, []string{"ack", "alert-2", "--reason=none", "--format", teleport.YAML})
	require.NoError(t, err)

	// transcode yaml to json, since AlertAcknowledgement does not contain yaml tags
	// for safe decoding
	yamlBuf := mustTranscodeYAMLToJSON(t, buf)
	out = mustDecodeJSON[types.AlertAcknowledgement](t, bytes.NewReader(yamlBuf))
	require.Equal(t, "alert-2", out.AlertID)

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
