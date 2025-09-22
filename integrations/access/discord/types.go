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

package discord

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
	} `json:"author"`
}
