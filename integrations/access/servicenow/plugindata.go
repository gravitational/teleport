/*
Copyright 2020-2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package servicenow

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

// DecodePluginData deserializes a string map to PluginData struct.
func DecodePluginData(dataMap map[string]string) (data PluginData, err error) {
	data.User = dataMap["user"]
	if str := dataMap["roles"]; str != "" {
		data.Roles = strings.Split(str, ",")
	}
	if str := dataMap["created"]; str != "" {
		created, err := strconv.ParseInt(dataMap["created"], 10, 64)
		if err != nil {
			return PluginData{}, trace.Wrap(err)
		}
		data.Created = time.Unix(created, 0)
	}
	data.RequestReason = dataMap["request_reason"]
	if str := dataMap["reviews_count"]; str != "" {
		fmt.Sscanf(str, "%d", &data.ReviewsCount)
	}
	data.Resolution.CloseCode = dataMap["close_code"]
	data.Resolution.State = dataMap["state"]
	data.Resolution.Reason = dataMap["resolve_reason"]
	data.IncidentID = dataMap["incident_id"]
	return
}

// EncodePluginData serializes a PluginData struct into a string map.
func EncodePluginData(data PluginData) map[string]string {
	result := make(map[string]string)
	result["incident_id"] = data.IncidentID
	result["user"] = data.User
	result["roles"] = strings.Join(data.Roles, ",")

	var createdStr string
	if !data.Created.IsZero() {
		createdStr = fmt.Sprintf("%d", data.Created.Unix())
	}
	result["created"] = createdStr

	result["request_reason"] = data.RequestReason

	var reviewsCountStr string
	if data.ReviewsCount != 0 {
		reviewsCountStr = fmt.Sprintf("%d", data.ReviewsCount)
	}
	result["reviews_count"] = reviewsCountStr

	result["close_code"] = data.Resolution.CloseCode
	result["state"] = data.Resolution.State
	result["resolve_reason"] = data.Resolution.Reason
	return result
}
