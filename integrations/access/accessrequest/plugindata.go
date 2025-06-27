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

package accessrequest

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/integrations/lib/plugindata"
)

// PluginData is a data associated with access request that we store in Teleport using UpdatePluginData API.
type PluginData struct {
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

// DecodePluginData deserializes a string map to PluginData struct.
func DecodePluginData(dataMap map[string]string) (PluginData, error) {
	data := PluginData{}

	var err error
	data.AccessRequestData, err = plugindata.DecodeAccessRequestData(dataMap)
	if err != nil {
		return PluginData{}, trace.Wrap(err)
	}

	if channelID, timestamp := dataMap["channel_id"], dataMap["timestamp"]; channelID != "" && timestamp != "" {
		data.SentMessages = append(data.SentMessages, MessageData{ChannelID: channelID, MessageID: timestamp})
	}

	if str := dataMap["messages"]; str != "" {
		for encodedMsg := range strings.SplitSeq(str, ",") {
			if parts := strings.Split(encodedMsg, "/"); len(parts) == 2 {
				data.SentMessages = append(data.SentMessages, MessageData{ChannelID: parts[0], MessageID: parts[1]})
			}
		}
	}
	return data, nil
}

// EncodePluginData serializes a PluginData struct into a string map.
func EncodePluginData(data PluginData) (map[string]string, error) {
	result, err := plugindata.EncodeAccessRequestData(data.AccessRequestData)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var encodedMessages []string
	for _, msg := range data.SentMessages {
		// TODO(hugoShaka): switch to base64 encode to avoid having / and , characters that could lead to bad parsing
		encodedMessages = append(encodedMessages, fmt.Sprintf("%s/%s", msg.ChannelID, msg.MessageID))
	}
	result["messages"] = strings.Join(encodedMessages, ",")

	return result, nil
}
