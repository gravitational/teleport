// Copyright 2024 Gravitational, Inc
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

package msteams

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/integrations/lib/plugindata"
)

var samplePluginData = PluginData{
	AccessRequestData: plugindata.AccessRequestData{
		User:             "user-foo",
		Roles:            []string{"role-foo", "role-bar"},
		RequestReason:    "foo reason",
		ReviewsCount:     3,
		ResolutionTag:    plugindata.ResolvedApproved,
		ResolutionReason: "foo ok",
	},
	TeamsData: []TeamsMessage{
		{ID: "CHANNEL1", Timestamp: "0000001", RecipientID: "foo@example.com"},
		{ID: "CHANNEL2", Timestamp: "0000002", RecipientID: "2ca235ec-37d0-44b0-964d-ca359e770603"},
		{ID: "CHANNEL3", Timestamp: "0000003", RecipientID: "https://teams.microsoft.com/l/channel/19%3af09f38d6d1594065862b1ca4a417319e%40thread.tacv2/Approval%2520Channel%25203?groupId=f2b3c8ed-5502-4449-b76f-dc3acea81f1c&tenantId=ff882432-09b0-437b-bd22-ca13c0037ded"},
	},
}

const messageData = "eyJpZCI6IkNIQU5ORUwxIiwidHMiOiIwMDAwMDAxIiwicmlkIjoiZm9vQGV4YW1wbGUuY29tIn0=,eyJpZCI6IkNIQU5ORUwyIiwidHMiOiIwMDAwMDAyIiwicmlkIjoiMmNhMjM1ZWMtMzdkMC00NGIwLTk2NGQtY2EzNTllNzcwNjAzIn0=,eyJpZCI6IkNIQU5ORUwzIiwidHMiOiIwMDAwMDAzIiwicmlkIjoiaHR0cHM6Ly90ZWFtcy5taWNyb3NvZnQuY29tL2wvY2hhbm5lbC8xOSUzYWYwOWYzOGQ2ZDE1OTQwNjU4NjJiMWNhNGE0MTczMTllJTQwdGhyZWFkLnRhY3YyL0FwcHJvdmFsJTI1MjBDaGFubmVsJTI1MjAzP2dyb3VwSWQ9ZjJiM2M4ZWQtNTUwMi00NDQ5LWI3NmYtZGMzYWNlYTgxZjFjXHUwMDI2dGVuYW50SWQ9ZmY4ODI0MzItMDliMC00MzdiLWJkMjItY2ExM2MwMDM3ZGVkIn0="

func TestEncodePluginData(t *testing.T) {
	dataMap, err := EncodePluginData(samplePluginData)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(dataMap), 8)
	assert.Equal(t, "user-foo", dataMap["user"])
	assert.Equal(t, "role-foo,role-bar", dataMap["roles"])
	assert.Equal(t, "foo reason", dataMap["request_reason"])
	assert.Equal(t, "3", dataMap["reviews_count"])
	assert.Equal(t, "APPROVED", dataMap["resolution"])
	assert.Equal(t, "foo ok", dataMap["resolve_reason"])
	assert.Empty(t, dataMap["resources"])
	assert.Equal(
		t,
		messageData,
		dataMap["messages"])
}

func TestDecodePluginDataCompatibility(t *testing.T) {
	pluginData, err := DecodePluginData(map[string]string{
		"user":           "user-foo",
		"roles":          "role-foo,role-bar",
		"request_reason": "foo reason",
		"reviews_count":  "3",
		"resolution":     "APPROVED",
		"resolve_reason": "foo ok",
		"messages":       "CHANNEL1/0000001/foo@example.com,CHANNEL2/0000002/2ca235ec-37d0-44b0-964d-ca359e770603",
	})
	assert.NoError(t, err)
	assert.Equal(t, samplePluginData.AccessRequestData, pluginData.AccessRequestData)
	// Legacy way of encoding messages does not support recipients containing '/' or ','
	// Hence we don't test the CHANNEL3
	assert.Equal(t, samplePluginData.TeamsData[0], pluginData.TeamsData[0])
	assert.Equal(t, samplePluginData.TeamsData[1], pluginData.TeamsData[1])
}

func TestDecodePluginData(t *testing.T) {
	pluginData, err := DecodePluginData(map[string]string{
		"user":           "user-foo",
		"roles":          "role-foo,role-bar",
		"request_reason": "foo reason",
		"reviews_count":  "3",
		"resolution":     "APPROVED",
		"resolve_reason": "foo ok",
		"messages":       messageData,
	})
	assert.NoError(t, err)
	assert.Equal(t, samplePluginData, pluginData)
}

func TestEncodeEmptyPluginData(t *testing.T) {
	dataMap, err := EncodePluginData(PluginData{})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(dataMap), 8)
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
