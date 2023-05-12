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

package discord

type AccessResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresInSeconds int    `json:"expires_in"`
}

type DiscordResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

type ChatMsgResponse struct {
	DiscordResponse
	Channel   string `json:"channel_id"`
	Text      string `json:"content"`
	DiscordID string `json:"id"`
}

type Msg struct {
	Channel   string `json:"channel,omitempty"`
	User      string `json:"user,omitempty"`
	Username  string `json:"username,omitempty"`
	DiscordID string `json:"id,omitempty"`
}

type DiscordMsg struct {
	Msg
	Text   string         `json:"content,omitempty"`
	Embeds []DiscordEmbed `json:"embeds,omitempty"`
}

type DiscordEmbed struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Color       int    `json:"color,omitempty"`
	Author      struct {
		Name string `json:"name"`
	} `json:"author,omitempty"`
}
