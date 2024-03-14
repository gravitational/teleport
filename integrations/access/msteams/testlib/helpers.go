package testlib

import (
	"context"
	"sort"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/access/msteams"
)

func (s *MsTeamsSuite) checkPluginData(ctx context.Context, reqID string, cond func(msteams.PluginData) bool) msteams.PluginData {
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

func (s *MsTeamsSuite) getNewMessages(ctx context.Context, n int) (MsgSlice, error) {
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
