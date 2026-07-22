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

package mattermost

import (
	"encoding/json"
	"fmt"
)

// Mattermost API types

type Props map[string]any

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
	URL     string         `json:"url,omitempty"`
	Context map[string]any `json:"context,omitempty"`
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
	if slice, ok := post.Props["attachments"].([]any); ok {
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
