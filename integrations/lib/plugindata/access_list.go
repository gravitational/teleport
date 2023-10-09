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
	"time"
)

// AccessListNotificationData represents generic plugin data required for access list notifications
type AccessListNotificationData struct {
	User             string
	LastNotification time.Time
}

// DecodeAccessListNotificationData deserializes a string map to PluginData struct.
func DecodeAccessListNotificationData(dataMap map[string]string) (data AccessListNotificationData, err error) {
	data.User = dataMap["user"]
	if dataMap["last_notification"] != "" {
		data.LastNotification, err = time.Parse(time.RFC3339Nano, dataMap["last_notification"])
	}
	return
}

// EncodeAccessListNotificationData deserializes a string map to PluginData struct.
func EncodeAccessListNotificationData(data AccessListNotificationData) (map[string]string, error) {
	result := make(map[string]string)
	result["user"] = data.User
	if !data.LastNotification.IsZero() {
		result["last_notification"] = data.LastNotification.Format(time.RFC3339Nano)
	} else {
		result["last_notification"] = ""
	}
	return result, nil
}
