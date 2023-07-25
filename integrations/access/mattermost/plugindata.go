/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mattermost

import (
	"fmt"
	"strings"
)

// PluginData is a data associated with access request that we store in Teleport using UpdatePluginData API.
type PluginData struct {
	RequestData
	MattermostData
}

type Resolution struct {
	Tag    ResolutionTag
	Reason string
}
type ResolutionTag string

const Unresolved = ResolutionTag("")
const ResolvedApproved = ResolutionTag("APPROVED")
const ResolvedDenied = ResolutionTag("DENIED")
const ResolvedExpired = ResolutionTag("EXPIRED")

type RequestData struct {
	User          string
	Roles         []string
	RequestReason string
	ReviewsCount  int
	Resolution    Resolution
}

type MattermostDataPost struct {
	PostID    string
	ChannelID string
}

type MattermostData = []MattermostDataPost

// DecodePluginData deserializes a string map to PluginData struct.
func DecodePluginData(dataMap map[string]string) (data PluginData) {
	data.User = dataMap["user"]
	if str := dataMap["roles"]; str != "" {
		data.Roles = strings.Split(str, ",")
	}
	data.RequestReason = dataMap["request_reason"]
	if str := dataMap["reviews_count"]; str != "" {
		fmt.Sscanf(str, "%d", &data.ReviewsCount)
	}
	data.Resolution.Tag = ResolutionTag(dataMap["resolution"])
	data.Resolution.Reason = dataMap["resolve_reason"]
	if channelID, postID := dataMap["channel_id"], dataMap["postID"]; channelID != "" && postID != "" {
		data.MattermostData = append(data.MattermostData, MattermostDataPost{ChannelID: channelID, PostID: postID})
	}
	if str := dataMap["messages"]; str != "" {
		for _, encodedMsg := range strings.Split(str, ",") {
			if parts := strings.Split(encodedMsg, "/"); len(parts) == 2 {
				data.MattermostData = append(data.MattermostData, MattermostDataPost{ChannelID: parts[0], PostID: parts[1]})
			}
		}
	}
	return
}

// EncodePluginData serializes a PluginData struct into a string map.
func EncodePluginData(data PluginData) map[string]string {
	result := make(map[string]string)

	result["user"] = data.User
	result["roles"] = strings.Join(data.Roles, ",")
	result["request_reason"] = data.RequestReason
	var reviewsCountStr string
	if data.ReviewsCount > 0 {
		reviewsCountStr = fmt.Sprintf("%d", data.ReviewsCount)
	}
	result["reviews_count"] = reviewsCountStr
	result["resolution"] = string(data.Resolution.Tag)
	result["resolve_reason"] = data.Resolution.Reason
	var encodedMessages []string
	for _, msg := range data.MattermostData {
		encodedMessages = append(encodedMessages, fmt.Sprintf("%s/%s", msg.ChannelID, msg.PostID))
	}
	result["messages"] = strings.Join(encodedMessages, ",")

	return result
}
