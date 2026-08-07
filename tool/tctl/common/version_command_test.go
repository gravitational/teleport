/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

	"github.com/gravitational/teleport/lib/utils"
)

func TestVersionInfoStructuredOutput(t *testing.T) {
	info := newVersionInfo()
	require.NotEmpty(t, info.Runtime)

	var jsonBuf bytes.Buffer
	require.NoError(t, utils.WriteJSON(&jsonBuf, info))
	gotJSON := mustDecodeJSON[versionInfo](t, &jsonBuf)
	require.Equal(t, info, gotJSON)

	var yamlBuf bytes.Buffer
	require.NoError(t, utils.WriteYAML(&yamlBuf, info))
	gotYAML := mustDecodeJSON[versionInfo](t, bytes.NewReader(mustTranscodeYAMLToJSON(t, &yamlBuf)))
	require.Equal(t, info, gotYAML)
}
