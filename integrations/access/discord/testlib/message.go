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

package testlib

import (
	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/discord"
)

type MessageSlice []discord.DiscordMsg
type MessageSet map[accessrequest.MessageData]struct{}

func (slice MessageSlice) Len() int {
	return len(slice)
}

func (slice MessageSlice) Less(i, j int) bool {
	if slice[i].Channel < slice[j].Channel {
		return true
	}
	return slice[i].DiscordID < slice[j].DiscordID
}

func (slice MessageSlice) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func (set MessageSet) Add(msg accessrequest.MessageData) {
	set[msg] = struct{}{}
}

func (set MessageSet) Contains(msg accessrequest.MessageData) bool {
	_, ok := set[msg]
	return ok
}
