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

package mattermost

import (
	"encoding/json"
	"fmt"
)

type AccessResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresInSeconds int    `json:"expires_in"`
}

// Mattermost API types

type Props map[string]interface{}

type Post struct {
	ID        string `json:"id,omitempty"`
	ChannelID string `json:"channel_id"`
	Message   string `json:"message"`
	RootID    string `json:"root_id"`
	Props     Props  `json:"props"`
}

type Attachment struct {
	ID      int64        `json:"id"`
	Text    string       `json:"text"`
	Actions []PostAction `json:"actions,omitempty"`
}

type PostAction struct {
	ID          string                 `json:"id,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Integration *PostActionIntegration `json:"integration,omitempty"`
}

type PostActionIntegration struct {
	URL     string                 `json:"url,omitempty"`
	Context map[string]interface{} `json:"context,omitempty"`
}

type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Channel struct {
	ID     string `json:"id"`
	TeamID string `json:"team_id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
}

type ErrorResult struct {
	ID            string `json:"id"`
	Message       string `json:"message"`
	DetailedError string `json:"detailed_error"`
	RequestID     string `json:"request_id"`
	StatusCode    int    `json:"status_code"`
}

func (e ErrorResult) Error() string {
	return fmt.Sprintf("api error status_code=%v, message=%q", e.StatusCode, e.Message)
}

func (post Post) Attachments() []Attachment {
	var attachments []Attachment
	if slice, ok := post.Props["attachments"].([]interface{}); ok {
		for _, dec := range slice {
			if enc, err := json.Marshal(dec); err == nil {
				var attachment Attachment
				if json.Unmarshal(enc, &attachment) == nil {
					attachments = append(attachments, attachment)
				}
			}
		}
	}
	return attachments
}
