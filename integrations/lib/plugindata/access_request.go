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
	"fmt"
	"strings"
)

// ResolutionTag represents enum type of access request resolution constant
type ResolutionTag string

const (
	Unresolved       = ResolutionTag("")
	ResolvedApproved = ResolutionTag("APPROVED")
	ResolvedDenied   = ResolutionTag("DENIED")
	ResolvedExpired  = ResolutionTag("EXPIRED")
)

// AccessRequestData represents generic plugin data required for access request processing
type AccessRequestData struct {
	User               string
	Roles              []string
	RequestReason      string
	ReviewsCount       int
	ResolutionTag      ResolutionTag
	ResolutionReason   string
	ResolveAnnotations map[string][]string
}

// DecodeAccessRequestData deserializes a string map to PluginData struct.
func DecodeAccessRequestData(dataMap map[string]string) (data AccessRequestData) {
	data.User = dataMap["user"]
	if str := dataMap["roles"]; str != "" {
		data.Roles = strings.Split(str, ",")
	}
	data.RequestReason = dataMap["request_reason"]
	if str := dataMap["reviews_count"]; str != "" {
		fmt.Sscanf(str, "%d", &data.ReviewsCount)
	}
	data.ResolutionTag = ResolutionTag(dataMap["resolution"])
	data.ResolutionReason = dataMap["resolve_reason"]

	return
}

// EncodeAccessRequestData deserializes a string map to PluginData struct.
func EncodeAccessRequestData(data AccessRequestData) map[string]string {
	result := make(map[string]string)

	result["user"] = data.User
	result["roles"] = strings.Join(data.Roles, ",")
	result["request_reason"] = data.RequestReason

	var reviewsCountStr string
	if data.ReviewsCount > 0 {
		reviewsCountStr = fmt.Sprintf("%d", data.ReviewsCount)
	}
	result["reviews_count"] = reviewsCountStr
	result["resolution"] = string(data.ResolutionTag)
	result["resolve_reason"] = data.ResolutionReason

	return result
}
