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

package testlib

import (
	"context"
	"sort"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/access/msteams"
)

func (s *MsTeamsBaseSuite) checkPluginData(ctx context.Context, reqID string, cond func(msteams.PluginData) bool) msteams.PluginData {
	t := s.T()
	t.Helper()

	for {
		rawData, err := s.Ruler().PollAccessRequestPluginData(ctx, "msteams", reqID)
		require.NoError(t, err)
		data, err := msteams.DecodePluginData(rawData)
		require.NoError(t, err)
		if cond(data) {
			return data
		}
	}
}

func (s *MsTeamsBaseSuite) getNewMessages(ctx context.Context, n int) (MsgSlice, error) {
	msgs := MsgSlice{}
	for i := 0; i < 2; i++ {
		msg, err := s.fakeTeams.CheckNewMessage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		msgs = append(msgs, msg)
	}
	sort.Sort(msgs)
	return msgs, nil
}
