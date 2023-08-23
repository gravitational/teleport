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

package plugindata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var sampleAccessRequestData = AccessRequestData{
	User:             "user-foo",
	Roles:            []string{"role-foo", "role-bar"},
	RequestReason:    "foo reason",
	ReviewsCount:     3,
	ResolutionTag:    ResolvedApproved,
	ResolutionReason: "foo ok",
}

func TestEncodeAccessRequestData(t *testing.T) {
	dataMap := EncodeAccessRequestData(sampleAccessRequestData)
	assert.Len(t, dataMap, 6)
	assert.Equal(t, "user-foo", dataMap["user"])
	assert.Equal(t, "role-foo,role-bar", dataMap["roles"])
	assert.Equal(t, "foo reason", dataMap["request_reason"])
	assert.Equal(t, "3", dataMap["reviews_count"])
	assert.Equal(t, "APPROVED", dataMap["resolution"])
	assert.Equal(t, "foo ok", dataMap["resolve_reason"])
}

func TestDecodeAccessRequestData(t *testing.T) {
	pluginData := DecodeAccessRequestData(map[string]string{
		"user":           "user-foo",
		"roles":          "role-foo,role-bar",
		"request_reason": "foo reason",
		"reviews_count":  "3",
		"resolution":     "APPROVED",
		"resolve_reason": "foo ok",
	})
	assert.Equal(t, sampleAccessRequestData, pluginData)
}

func TestEncodeEmptyAccessRequestData(t *testing.T) {
	dataMap := EncodeAccessRequestData(AccessRequestData{})
	assert.Len(t, dataMap, 6)
	for key, value := range dataMap {
		assert.Emptyf(t, value, "value at key %q must be empty", key)
	}
}

func TestDecodeEmptyAccessRequestData(t *testing.T) {
	assert.Empty(t, DecodeAccessRequestData(nil))
	assert.Empty(t, DecodeAccessRequestData(make(map[string]string)))
}
