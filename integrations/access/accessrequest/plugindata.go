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
	"encoding/base64"
	"encoding/json"
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
	ChannelID string `json:"channelId"`
	// MessageID identifies a specific message in the channel.
	// For example: on Discord this is an ID while on Slack this is a timestamp.
	MessageID string `json:"messageId"`
}

type SentMessages []MessageData

// DecodePluginData deserializes a string map to PluginData struct.
func DecodePluginData(dataMap map[string]string) (PluginData, error) {
	data := PluginData{}
	var errors []error

	accessRequestData, err := plugindata.DecodeAccessRequestData(dataMap)
	if err != nil {
		return data, trace.Wrap(err, "failed to decode access request data")
	}
	data.AccessRequestData = accessRequestData

	// Backward compatibility for single-message data
	if channelID, timestamp := dataMap["channel_id"], dataMap["timestamp"]; channelID != "" && timestamp != "" {
		data.SentMessages = append(data.SentMessages, MessageData{ChannelID: channelID, MessageID: timestamp})
	}

	if str := dataMap["messages"]; str != "" {
		for encodedMsg := range strings.SplitSeq(str, ",") {
			var msg MessageData
			decodedMsg, err := base64.StdEncoding.DecodeString(encodedMsg)
			if err == nil {
				err = json.Unmarshal(decodedMsg, &msg)
			}
			if err != nil {
				// Backward compatibility
				parts := strings.Split(encodedMsg, "/")
				if len(parts) == 2 {
					data.SentMessages = append(data.SentMessages, MessageData{ChannelID: parts[0], MessageID: parts[1]})
				} else {
					errors = append(errors, err)
				}
				continue
			}
			data.SentMessages = append(data.SentMessages, msg)
		}
	}
	return data, trace.NewAggregate(errors...)
}

// EncodePluginData serializes a PluginData struct into a string map.
func EncodePluginData(data PluginData) (map[string]string, error) {
	result, err := plugindata.EncodeAccessRequestData(data.AccessRequestData)
	if err != nil {
		return nil, trace.Wrap(err, "failed to encode access request data")
	}

	var errors []error
	var encodedMessages []string
	for _, msg := range data.SentMessages {
		jsonMessage, err := json.Marshal(msg)
		if err != nil {
			errors = append(errors, err)
		}
		encodedMessage := base64.StdEncoding.EncodeToString(jsonMessage)
		encodedMessages = append(encodedMessages, encodedMessage)
	}
	result["messages"] = strings.Join(encodedMessages, ",")

	return result, trace.NewAggregate(errors...)
}
