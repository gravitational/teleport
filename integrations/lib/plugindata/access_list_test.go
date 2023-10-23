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
	UserNotifications: map[string]time.Time{
		"user-foo":   time.Now().UTC(),
		"user-foo-2": time.Now().UTC(),
	},
}

func TestEncodeAccessListNotificationData(t *testing.T) {
	dataMap, err := EncodeAccessListNotificationData(sampleAccessListNotificationData)
	assert.NoError(t, err)
	assert.Len(t, dataMap, 2)
	assert.Equal(t, map[string]string{
		"un_user-foo":   sampleAccessListNotificationData.UserNotifications["user-foo"].Format(time.RFC3339Nano),
		"un_user-foo-2": sampleAccessListNotificationData.UserNotifications["user-foo-2"].Format(time.RFC3339Nano),
	}, dataMap)
}

func TestDecodeAccessListNotificationData(t *testing.T) {
	pluginData, err := DecodeAccessListNotificationData(map[string]string{
		"un_user-foo":   sampleAccessListNotificationData.UserNotifications["user-foo"].Format(time.RFC3339Nano),
		"un_user-foo-2": sampleAccessListNotificationData.UserNotifications["user-foo-2"].Format(time.RFC3339Nano),
	})
	assert.NoError(t, err)
	assert.Equal(t, sampleAccessListNotificationData, pluginData)
}

func TestEncodeEmptyAccessListNotificationtData(t *testing.T) {
	dataMap, err := EncodeAccessListNotificationData(AccessListNotificationData{})
	assert.NoError(t, err)
	assert.Empty(t, dataMap)
}

func TestDecodeEmptyAccessListNotificationtData(t *testing.T) {
	decoded, err := DecodeAccessListNotificationData(nil)
	assert.NoError(t, err)
	assert.Empty(t, decoded)
	decoded, err = DecodeAccessListNotificationData(make(map[string]string))
	assert.NoError(t, err)
	assert.Empty(t, decoded)
}
