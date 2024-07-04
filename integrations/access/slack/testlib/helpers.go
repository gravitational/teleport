/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/slack"
)

func (s *SlackBaseSuite) checkPluginData(ctx context.Context, reqID string, cond func(accessrequest.PluginData) bool) accessrequest.PluginData {
	t := s.T()
	t.Helper()

	for {
		rawData, err := s.Ruler().PollAccessRequestPluginData(ctx, "slack", reqID)
		require.NoError(t, err)
		data, err := accessrequest.DecodePluginData(rawData)
		require.NoError(t, err)
		if cond(data) {
			return data
		}
	}
}

var msgFieldRegexp = regexp.MustCompile(`(?im)^\*([a-zA-Z ]+)\*: (.+)$`)
var requestReasonRegexp = regexp.MustCompile("(?im)^\\*Reason\\*:\\ ```\\n(.*?)```(.*?)$")

func parseMessageField(msg slack.Message, field string) (string, error) {
	block := msg.BlockItems[1].Block
	sectionBlock, ok := block.(slack.SectionBlock)
	if !ok {
		return "", trace.Errorf("invalid block type %T", block)
	}

	if sectionBlock.Text.TextObject == nil {
		return "", trace.Errorf("section block does not contain text")
	}

	text := sectionBlock.Text.GetText()
	matches := msgFieldRegexp.FindAllStringSubmatch(text, -1)
	if matches == nil {
		return "", trace.Errorf("cannot parse fields from text %s", text)
	}
	var fields []string
	for _, match := range matches {
		if match[1] == field {
			return match[2], nil
		}
		fields = append(fields, match[1])
	}
	return "", trace.Errorf("cannot find field %s in %v", field, fields)
}

func getStatusLine(msg slack.Message) (string, error) {
	block := msg.BlockItems[2].Block
	contextBlock, ok := block.(slack.ContextBlock)
	if !ok {
		return "", trace.Errorf("invalid block type %T", block)
	}

	elementItems := contextBlock.ElementItems
	if n := len(elementItems); n != 1 {
		return "", trace.Errorf("expected only one context element, got %v", n)
	}

	element := elementItems[0].ContextElement
	textBlock, ok := element.(slack.TextObject)
	if !ok {
		return "", trace.Errorf("invalid element type %T", element)
	}

	return textBlock.GetText(), nil
}

// matchFns are functions that tell how to match two messages together after matching on the channel ID.
type matchFn func(matchAgainst, newMsg slack.Message) bool

func matchOnlyOnChannel(_, _ slack.Message) bool {
	return true
}

func matchByTimestamp(matchAgainst, newMsg slack.Message) bool {
	return matchAgainst.Timestamp == newMsg.Timestamp
}

func matchByThreadTs(matchAgainst, newMsg slack.Message) bool {
	return matchAgainst.Timestamp == newMsg.ThreadTs
}

// checkMsgTestFn is a test function to run on a new message after it has been matched.
type checkMsgTestFn func(*testing.T, slack.Message)

func (s *SlackBaseSuite) checkNewMessages(t *testing.T, ctx context.Context, matchMessages []slack.Message, matchBy matchFn, testFns ...checkMsgTestFn) []slack.Message {
	t.Helper()
	return s.matchAndCallFn(t, ctx, matchMessages, matchBy, testFns, s.fakeSlack.CheckNewMessage)
}

func (s *SlackBaseSuite) checkNewMessageUpdateByAPI(t *testing.T, ctx context.Context, matchMessages []slack.Message, matchBy matchFn, testFns ...checkMsgTestFn) []slack.Message {
	t.Helper()
	return s.matchAndCallFn(t, ctx, matchMessages, matchBy, testFns, s.fakeSlack.CheckMessageUpdateByAPI)
}

func channelsToMessages(channels ...string) (messages []slack.Message) {
	for _, channel := range channels {
		messages = append(messages, slack.Message{BaseMessage: slack.BaseMessage{Channel: channel}})
	}

	return messages
}

type slackCheckMessage func(context.Context) (slack.Message, error)

func (s *SlackBaseSuite) matchAndCallFn(t *testing.T, ctx context.Context, matchMessages []slack.Message, matchBy matchFn, testFns []checkMsgTestFn, slackCall slackCheckMessage) []slack.Message {
	s.T().Helper()

	matchingTimestamps := map[string]slack.Message{}

	for _, matchMessage := range matchMessages {
		matchingTimestamps[matchMessage.Channel] = matchMessage
	}

	var messages []slack.Message
	var notMatchingMessages []slack.Message

	// Try for 5 seconds to get the expected messages
	require.Eventually(t, func() bool {
		msg, err := slackCall(ctx)
		if err != nil {
			return false
		}

		if matchMsg, ok := matchingTimestamps[msg.Channel]; ok {
			if matchBy(matchMsg, msg) {
				messages = append(messages, msg)
			}
		} else {
			notMatchingMessages = append(notMatchingMessages, msg)
		}

		return len(messages) == len(matchMessages)
	}, 2*time.Second, 100*time.Millisecond)

	require.Len(t, messages, len(matchMessages), "missing required messages, found %v", notMatchingMessages)

	for _, testFn := range testFns {
		for _, message := range messages {
			testFn(t, message)
		}
	}

	return messages
}
