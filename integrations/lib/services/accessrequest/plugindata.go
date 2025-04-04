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
		for _, encodedMsg := range strings.Split(str, ",") {
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
