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

	return data, err
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
