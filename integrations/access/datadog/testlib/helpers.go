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
	"regexp"

	"github.com/gravitational/trace"
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

var summaryFieldRegexp = regexp.MustCompile(`([a-zA-Z ]+): (.+)`)

func parseSummaryField(incidentSumamry string, field string) (string, error) {
	matches := summaryFieldRegexp.FindAllStringSubmatch(incidentSumamry, -1)
	if matches == nil {
		return "", trace.Errorf("cannot parse fields from text %s", incidentSumamry)
	}
	var fields []string
	for _, match := range matches {
		if match[1] == field {
			return match[2], nil
		}
		fields = append(fields, match[1])
	}
	return "", trace.Errorf("cannot find field %s in %v", field, fields)
}
