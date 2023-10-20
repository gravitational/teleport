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
	"strings"
	"time"
)

// AccessListNotificationData represents generic plugin data required for access list notifications
type AccessListNotificationData struct {
	UserNotifications map[string]time.Time
}

// DecodeAccessListNotificationData deserializes a string map to PluginData struct.
func DecodeAccessListNotificationData(dataMap map[string]string) (data AccessListNotificationData, err error) {
	for user, notification := range dataMap {
		if strings.HasPrefix(user, "un_") {
			if data.UserNotifications == nil {
				data.UserNotifications = map[string]time.Time{}
			}
			notificationTime, err := time.Parse(time.RFC3339Nano, notification)
			if err != nil {
				return data, err
			}
			data.UserNotifications[strings.TrimPrefix(user, "un_")] = notificationTime
		}
	}

	return
}

// EncodeAccessListNotificationData deserializes a string map to PluginData struct.
func EncodeAccessListNotificationData(data AccessListNotificationData) (map[string]string, error) {
	result := make(map[string]string)
	for user, notificationTime := range data.UserNotifications {
		result["un_"+user] = notificationTime.Format(time.RFC3339Nano)
	}
	return result, nil
}
