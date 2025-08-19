// Copyright 2024 Gravitational, Inc
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

package msteams

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
	TeamsData []TeamsMessage
}

// TeamsMessage represents sent message information
type TeamsMessage struct {
	ID          string `json:"id"`
	Timestamp   string `json:"ts"`
	RecipientID string `json:"rid"`
}

// DecodePluginData deserializes a string map to PluginData struct.
func DecodePluginData(dataMap map[string]string) (PluginData, error) {
	data := PluginData{}
	var errors []error

	accessRequestData, err := plugindata.DecodeAccessRequestData(dataMap)
	if err != nil {
		return data, trace.Wrap(err, "failed to decode access request data")
	}
	data.AccessRequestData = accessRequestData

	if str := dataMap["messages"]; str != "" {
		for encodedMsg := range strings.SplitSeq(str, ",") {
			decodedMsg, err := base64.StdEncoding.DecodeString(encodedMsg)
			if err != nil {
				// Backward compatibility
				// TODO(hugoShaka): remove in v12
				parts := strings.Split(encodedMsg, "/")
				if len(parts) == 3 {
					data.TeamsData = append(data.TeamsData, TeamsMessage{ID: parts[0], Timestamp: parts[1], RecipientID: parts[2]})
				}
				continue
			}

			msg := &TeamsMessage{}
			err = json.Unmarshal(decodedMsg, msg)
			if err != nil {
				errors = append(errors, err)
			}
			data.TeamsData = append(data.TeamsData, *msg)
		}
	}

	return data, trace.NewAggregate(errors...)
}

// EncodePluginData serializes plugin data to a string map
func EncodePluginData(data PluginData) (map[string]string, error) {
	result, err := plugindata.EncodeAccessRequestData(data.AccessRequestData)
	if err != nil {
		return nil, trace.Wrap(err, "failed to encode access request data")
	}

	var errors []error

	var encodedMessages []string
	for _, msg := range data.TeamsData {
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
