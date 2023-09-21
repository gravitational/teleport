/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mattermost

import (
	"context"
	"sync/atomic"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
)

type MattermostPostSlice []Post
type MattermostDataPostSet map[common.MessageData]struct{}

func (slice MattermostPostSlice) Len() int {
	return len(slice)
}

func (slice MattermostPostSlice) Less(i, j int) bool {
	if slice[i].ChannelID < slice[j].ChannelID {
		return true
	}
	return slice[i].ID < slice[j].ID
}

func (slice MattermostPostSlice) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func (set MattermostDataPostSet) Add(msg common.MessageData) {
	set[msg] = struct{}{}
}

func (set MattermostDataPostSet) Contains(msg common.MessageData) bool {
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
