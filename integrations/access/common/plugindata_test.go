// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/integrations/lib/plugindata"
)

var samplePluginData = GenericPluginData{
	AccessRequestData: plugindata.AccessRequestData{
		User:             "user-foo",
		Roles:            []string{"role-foo", "role-bar"},
		RequestReason:    "foo reason",
		ReviewsCount:     3,
		ResolutionTag:    plugindata.ResolvedApproved,
		ResolutionReason: "foo ok",
	},
	SentMessages: SentMessages{
		{ChannelID: "CHANNEL1", MessageID: "0000001"},
		{ChannelID: "CHANNEL2", MessageID: "0000002"},
	},
}

func TestEncodePluginData(t *testing.T) {
	dataMap, err := EncodePluginData(samplePluginData)
	assert.NoError(t, err)
	assert.Len(t, dataMap, 7)
	assert.Equal(t, "user-foo", dataMap["user"])
	assert.Equal(t, "role-foo,role-bar", dataMap["roles"])
	assert.Equal(t, "foo reason", dataMap["request_reason"])
	assert.Equal(t, "3", dataMap["reviews_count"])
	assert.Equal(t, "APPROVED", dataMap["resolution"])
	assert.Equal(t, "foo ok", dataMap["resolve_reason"])
	assert.Equal(t, "CHANNEL1/0000001,CHANNEL2/0000002", dataMap["messages"])
}

func TestDecodePluginData(t *testing.T) {
	pluginData, err := DecodePluginData(map[string]string{
		"user":           "user-foo",
		"roles":          "role-foo,role-bar",
		"request_reason": "foo reason",
		"reviews_count":  "3",
		"resolution":     "APPROVED",
		"resolve_reason": "foo ok",
		"messages":       "CHANNEL1/0000001,CHANNEL2/0000002",
	})
	assert.NoError(t, err)
	assert.Equal(t, samplePluginData, pluginData)
}

func TestEncodeEmptyPluginData(t *testing.T) {
	dataMap, err := EncodePluginData(GenericPluginData{})
	assert.NoError(t, err)
	assert.Len(t, dataMap, 7)
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
