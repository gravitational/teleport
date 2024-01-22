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

package plugindata

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Len(t, dataMap, 1)

	userNotificationsData, err := json.Marshal(sampleAccessListNotificationData.UserNotifications)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"user_notifications": string(userNotificationsData),
	}, dataMap)
}

func TestDecodeAccessListNotificationData(t *testing.T) {
	userNotificationsData, err := json.Marshal(sampleAccessListNotificationData.UserNotifications)
	require.NoError(t, err)
	pluginData, err := DecodeAccessListNotificationData(map[string]string{
		"user_notifications": string(userNotificationsData),
	})
	assert.NoError(t, err)
	assert.Equal(t, sampleAccessListNotificationData, pluginData)
}

func TestEncodeEmptyAccessListNotificationtData(t *testing.T) {
	dataMap, err := EncodeAccessListNotificationData(AccessListNotificationData{})
	assert.NoError(t, err)
	assert.Len(t, dataMap, 1)
	assert.Empty(t, dataMap["userNotifications"])
}

func TestDecodeEmptyAccessListNotificationtData(t *testing.T) {
	decoded, err := DecodeAccessListNotificationData(nil)
	assert.NoError(t, err)
	assert.Empty(t, decoded)
	decoded, err = DecodeAccessListNotificationData(make(map[string]string))
	assert.NoError(t, err)
	assert.Empty(t, decoded)
}
