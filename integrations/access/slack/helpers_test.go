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

package slack

import (
	"context"
	"sync/atomic"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/accessrequest"
)

type SlackMessageSlice []Message
type SlackDataMessageSet map[accessrequest.MessageData]struct{}

func (slice SlackMessageSlice) Len() int {
	return len(slice)
}

func (slice SlackMessageSlice) Less(i, j int) bool {
	if slice[i].Channel < slice[j].Channel {
		return true
	}
	return slice[i].Timestamp < slice[j].Timestamp
}

func (slice SlackMessageSlice) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func (set SlackDataMessageSet) Add(msg accessrequest.MessageData) {
	set[msg] = struct{}{}
}

func (set SlackDataMessageSet) Contains(msg accessrequest.MessageData) bool {
	_, ok := set[msg]
	return ok
}

type fakeStatusSink struct {
	status atomic.Pointer[types.PluginStatus]
}

func (s *fakeStatusSink) Emit(_ context.Context, status types.PluginStatus) error {
	s.status.Store(&status)
	return nil
}

func (s *fakeStatusSink) Get() types.PluginStatus {
	status := s.status.Load()
	if status == nil {
		panic("expected status to be set, but it has not been")
	}
	return *status
}
