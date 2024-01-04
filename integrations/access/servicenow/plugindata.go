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

package servicenow

import (
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
		reviewsCount, err := strconv.Atoi(dataMap["reviews_count"])
		if err != nil {
			return PluginData{}, trace.Wrap(err)
		}
		data.ReviewsCount = reviewsCount
	}
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
		createdStr = strconv.FormatInt(data.Created.Unix(), 10)
	}
	result["created"] = createdStr

	result["request_reason"] = data.RequestReason

	var reviewsCountStr string
	if data.ReviewsCount != 0 {
		reviewsCountStr = strconv.Itoa(data.ReviewsCount)
	}
	result["reviews_count"] = reviewsCountStr

	result["state"] = data.Resolution.State
	result["resolve_reason"] = data.Resolution.Reason
	return result
}
