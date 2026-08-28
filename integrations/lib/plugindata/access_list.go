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
	"time"

	"github.com/gravitational/trace"
)

// AccessListNotificationData represents generic plugin data required for access list notifications
type AccessListNotificationData struct {
	UserNotifications map[string]time.Time
}

// DecodeAccessListNotificationData deserializes a string map to PluginData struct.
func DecodeAccessListNotificationData(dataMap map[string]string) (data AccessListNotificationData, err error) {
	userNotificationsData := dataMap["user_notifications"]
	if userNotificationsData != "" {
		if err := json.Unmarshal([]byte(userNotificationsData), &data.UserNotifications); err != nil {
			return data, trace.Wrap(err)
		}
	}

	return data, nil
}

// EncodeAccessListNotificationData deserializes a string map to PluginData struct.
func EncodeAccessListNotificationData(data AccessListNotificationData) (map[string]string, error) {
	result := make(map[string]string)

	result["user_notifications"] = ""

	if len(data.UserNotifications) > 0 {
		userNotificationsData, err := json.Marshal(data.UserNotifications)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result["user_notifications"] = string(userNotificationsData)
	}

	return result, nil
}
