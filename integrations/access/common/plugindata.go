/*
Copyright 2022 Gravitational, Inc.

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

package common

import (
	"fmt"
	"strings"

	"github.com/gravitational/teleport/integrations/lib/plugindata"
)

// GenericPluginData is a data associated with access request that we store in Teleport using UpdatePluginData API.
type GenericPluginData struct {
	plugindata.AccessRequestData
	SentMessages
}

// MessageData contains all the required information to identify and edit a message.
type MessageData struct {
	// ChannelID identifies a channel.
	ChannelID string
	// MessageID identifies a specific message in the channel.
	// For example: on Discord this is an ID while on Slack this is a timestamp.
	MessageID string
}

type SentMessages []MessageData

// DecodePluginData deserializes a string map to GenericPluginData struct.
func DecodePluginData(dataMap map[string]string) (GenericPluginData, error) {
	data := GenericPluginData{}

	data.AccessRequestData = plugindata.DecodeAccessRequestData(dataMap)

	if channelID, timestamp := dataMap["channel_id"], dataMap["timestamp"]; channelID != "" && timestamp != "" {
		data.SentMessages = append(data.SentMessages, MessageData{ChannelID: channelID, MessageID: timestamp})
	}

	if str := dataMap["messages"]; str != "" {
		for _, encodedMsg := range strings.Split(str, ",") {
			if parts := strings.Split(encodedMsg, "/"); len(parts) == 2 {
				data.SentMessages = append(data.SentMessages, MessageData{ChannelID: parts[0], MessageID: parts[1]})
			}
		}
	}
	return data, nil
}

// EncodePluginData serializes a GenericPluginData struct into a string map.
func EncodePluginData(data GenericPluginData) (map[string]string, error) {
	result := plugindata.EncodeAccessRequestData(data.AccessRequestData)

	var encodedMessages []string
	for _, msg := range data.SentMessages {
		// TODO(hugoShaka): switch to base64 encode to avoid having / and , characters that could lead to bad parsing
		encodedMessages = append(encodedMessages, fmt.Sprintf("%s/%s", msg.ChannelID, msg.MessageID))
	}
	result["messages"] = strings.Join(encodedMessages, ",")

	return result, nil
}
