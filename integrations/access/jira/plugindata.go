/*
Copyright 2021 Gravitational, Inc.

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

package jira

import (
	"fmt"
	"strings"
	"time"
)

// PluginData is a data associated with access request that we store in Teleport using UpdatePluginData API.
type PluginData struct {
	RequestData
	JiraData
}

type Resolution struct {
	Tag    ResolutionTag
	Reason string
}
type ResolutionTag string

const Unresolved = ResolutionTag("")
const ResolvedApproved = ResolutionTag("approved")
const ResolvedDenied = ResolutionTag("denied")
const ResolvedExpired = ResolutionTag("expired")

type RequestData struct {
	User          string
	Roles         []string
	Created       time.Time
	RequestReason string
	ReviewsCount  int
	Resolution    Resolution
}

type JiraData struct {
	IssueID  string
	IssueKey string
}

// DecodePluginData deserializes a string map to PluginData struct.
func DecodePluginData(dataMap map[string]string) (data PluginData) {
	data.User = dataMap["user"]
	if str := dataMap["roles"]; str != "" {
		data.Roles = strings.Split(str, ",")
	}
	if str := dataMap["created"]; str != "" {
		var created int64
		fmt.Sscanf(str, "%d", &created)
		data.Created = time.Unix(created, 0)
	}
	data.RequestReason = dataMap["request_reason"]
	if str := dataMap["reviews_count"]; str != "" {
		fmt.Sscanf(str, "%d", &data.ReviewsCount)
	}
	data.Resolution.Tag = ResolutionTag(dataMap["resolution"])
	data.Resolution.Reason = dataMap["resolve_reason"]
	data.IssueID = dataMap["issue_id"]
	data.IssueKey = dataMap["issue_key"]
	return
}

// EncodePluginData serializes a PluginData struct into a string map.
func EncodePluginData(data PluginData) map[string]string {
	result := make(map[string]string)

	result["issue_id"] = data.IssueID
	result["issue_key"] = data.IssueKey
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
