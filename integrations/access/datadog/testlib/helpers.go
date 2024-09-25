/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package testlib

import (
	"context"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/access/accessrequest"
)

func (s *DatadogBaseSuite) checkPluginData(ctx context.Context, reqID string, cond func(accessrequest.PluginData) bool) accessrequest.PluginData {
	t := s.T()
	t.Helper()

	for {
		rawData, err := s.Ruler().PollAccessRequestPluginData(ctx, "datadog", reqID)
		require.NoError(t, err)
		data, err := accessrequest.DecodePluginData(rawData)
		require.NoError(t, err)
		if cond(data) {
			return data
		}
	}
}
