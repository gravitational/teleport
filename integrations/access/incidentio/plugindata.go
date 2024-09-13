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

package incidentio

import (
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/types"
)

// PluginData is a data associated with access request that we store in Teleport using UpdatePluginData API.
type PluginData struct {
	RequestData
	IncidentAlertData
}

// Resolution stores the resolution (approved, denied or expired) and its reason.
type Resolution struct {
	Tag    ResolutionTag
	Reason string
}

// ResolutionTag represents if and in which state an access request was resolved.
type ResolutionTag string

// Unresolved is added to alerts that are yet to be resolved.
const Unresolved = ResolutionTag("")

// ResolvedApproved is added to alerts that are approved.
const ResolvedApproved = ResolutionTag("approved")

// ResolvedDenied is added to alerts that are denied.
const ResolvedDenied = ResolutionTag("denied")

// ResolvedExpired is added to alerts that are expired.
const ResolvedExpired = ResolutionTag("expired")

// ResolvedPromoted is added to alerts that are promoted to an access list.
const ResolvedPromoted = ResolutionTag("promoted")

// RequestData stores a slice of some request fields in a convenient format.
type RequestData struct {
	User              string
	Roles             []string
	Created           time.Time
	RequestReason     string
	ReviewsCount      int
	Resolution        Resolution
	SystemAnnotations types.Labels
}

// IncidentAlertData stores the deduplication key
type IncidentAlertData struct {
	DeduplicationKey string
}

// DecodePluginData deserializes a string map to PluginData struct.
func DecodePluginData(dataMap map[string]string) (data PluginData) {
	data.User = dataMap["user"]
	if str := dataMap["roles"]; str != "" {
		data.Roles = strings.Split(str, ",")
	}
	if str := dataMap["created"]; str != "" {
		var created int64
		fmt.Sscanf(dataMap["created"], "%d", &created)
		data.Created = time.Unix(created, 0)
	}
	data.RequestReason = dataMap["request_reason"]
	if str := dataMap["reviews_count"]; str != "" {
		fmt.Sscanf(str, "%d", &data.ReviewsCount)
	}
	data.Resolution.Tag = ResolutionTag(dataMap["resolution"])
	data.Resolution.Reason = dataMap["resolve_reason"]
	data.DeduplicationKey = dataMap["deduplication_key"]
	return
}

// EncodePluginData serializes a PluginData struct into a string map.
func EncodePluginData(data PluginData) map[string]string {
	result := make(map[string]string)
	result["deduplication_key"] = data.DeduplicationKey
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

	result["resolution"] = string(data.Resolution.Tag)
	result["resolve_reason"] = data.Resolution.Reason
	return result
}
