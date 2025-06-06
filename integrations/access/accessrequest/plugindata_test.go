/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package accessrequest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/lib/plugindata"
)

func getSamplePluginData(t *testing.T) PluginData {
	maxDuration, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	require.NoError(t, err)
	return PluginData{
		AccessRequestData: plugindata.AccessRequestData{
			User:             "user-foo",
			Roles:            []string{"role-foo", "role-bar"},
			Resources:        []string{"cluster-a/node/foo", "cluster-a/node/bar"},
			RequestReason:    "foo reason",
			ReviewsCount:     3,
			ResolutionTag:    plugindata.ResolvedApproved,
			ResolutionReason: "foo ok",
			MaxDuration:      &maxDuration,
		},
		SentMessages: SentMessages{
			{ChannelID: "CHANNEL1", MessageID: "0000001"},
			{ChannelID: "CHANNEL2", MessageID: "0000002"},
		},
	}
}

func TestEncodePluginData(t *testing.T) {
	dataMap, err := EncodePluginData(getSamplePluginData(t))
	assert.NoError(t, err)
	assert.Len(t, dataMap, 9)
	assert.Equal(t, "user-foo", dataMap["user"])
	assert.Equal(t, "role-foo,role-bar", dataMap["roles"])
	assert.Equal(t, `["cluster-a/node/foo","cluster-a/node/bar"]`, dataMap["resources"])
	assert.Equal(t, "foo reason", dataMap["request_reason"])
	assert.Equal(t, "3", dataMap["reviews_count"])
	assert.Equal(t, "APPROVED", dataMap["resolution"])
	assert.Equal(t, "foo ok", dataMap["resolve_reason"])
	assert.Equal(t, "CHANNEL1/0000001,CHANNEL2/0000002", dataMap["messages"])
	assert.Equal(t, "2006-01-02T15:04:05Z", dataMap["max_duration"])
}

func TestDecodePluginData(t *testing.T) {
	pluginData, err := DecodePluginData(map[string]string{
		"user":           "user-foo",
		"roles":          "role-foo,role-bar",
		"resources":      `["cluster-a/node/foo","cluster-a/node/bar"]`,
		"request_reason": "foo reason",
		"reviews_count":  "3",
		"resolution":     "APPROVED",
		"resolve_reason": "foo ok",
		"messages":       "CHANNEL1/0000001,CHANNEL2/0000002",
		"max_duration":   "2006-01-02T15:04:05Z",
	})
	assert.NoError(t, err)
	assert.Equal(t, getSamplePluginData(t), pluginData)
}

func TestEncodeEmptyPluginData(t *testing.T) {
	dataMap, err := EncodePluginData(PluginData{})
	assert.NoError(t, err)
	assert.Len(t, dataMap, 8)
	for key, value := range dataMap {
		assert.Emptyf(t, value, "value at key %q must be empty", key)
	}
}

func TestDecodeEmptyPluginData(t *testing.T) {
	result, err := DecodePluginData(nil)
	assert.NoError(t, err)
	assert.Empty(t, result)

	result, err = DecodePluginData(make(map[string]string))
	assert.NoError(t, err)
	assert.Empty(t, result)
}
