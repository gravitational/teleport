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
	"time"

	"github.com/stretchr/testify/assert"
)

var sampleAccessListNotificationData = AccessListNotificationData{
	User:             "user-foo",
	LastNotification: time.Now().UTC(),
}

func TestEncodeAccessListNotificationData(t *testing.T) {
	dataMap, err := EncodeAccessListNotificationData(sampleAccessListNotificationData)
	assert.Nil(t, err)
	assert.Len(t, dataMap, 2)
	assert.Equal(t, "user-foo", dataMap["user"])
	assert.Equal(t, sampleAccessListNotificationData.LastNotification.Format(time.RFC3339Nano), dataMap["last_notification"])
}

func TestDecodeAccessListNotificationData(t *testing.T) {
	pluginData, err := DecodeAccessListNotificationData(map[string]string{
		"user":              "user-foo",
		"last_notification": sampleAccessListNotificationData.LastNotification.Format(time.RFC3339Nano),
	})
	assert.Nil(t, err)
	assert.Equal(t, sampleAccessListNotificationData, pluginData)
}

func TestEncodeEmptyAccessListNotificationtData(t *testing.T) {
	dataMap, err := EncodeAccessListNotificationData(AccessListNotificationData{})
	assert.Nil(t, err)
	assert.Len(t, dataMap, 2)
	for key, value := range dataMap {
		assert.Emptyf(t, value, "value at key %q must be empty", key)
	}
}

func TestDecodeEmptyAccessListNotificationtData(t *testing.T) {
	decoded, err := DecodeAccessListNotificationData(nil)
	assert.Nil(t, err)
	assert.Empty(t, decoded)
	decoded, err = DecodeAccessListNotificationData(make(map[string]string))
	assert.Nil(t, err)
	assert.Empty(t, decoded)
}
