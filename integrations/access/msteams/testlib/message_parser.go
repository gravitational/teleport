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
	"testing"

	"github.com/stretchr/testify/require"
)

// testTeamsMessage is here only for tetsing purposes.
// As the adaptative cards library doesn't support unmarshalling, this is a
// struct containing a subset of the fields used in the tests.
type testTeamsMessage struct {
	Attachments []struct {
		Content struct {
			Body []struct {
				Text    string `json:"text,omitempty"`
				Columns []struct {
					Items []struct {
						Text string `json:"text"`
					} `json:"items"`
				} `json:"columns,omitempty"`
				Facts []struct {
					Value string `json:"value"`
				} `json:"facts,omitempty"`
			} `json:"body"`
		} `json:"content"`
	} `json:"attachments"`
}

func (msg testTeamsMessage) checkTitle(t *testing.T, title string) {
	t.Helper()
	require.Equal(t, title, msg.getTitle())
}

func (msg testTeamsMessage) checkStatusApproved(t *testing.T, reason string) {
	t.Helper()
	body := msg.Attachments[0].Content.Body
	require.GreaterOrEqual(t, len(body), 3)
	require.Equal(t, "✅", body[1].Columns[0].Items[0].Text)
	require.Equal(t, "APPROVED", body[1].Columns[1].Items[0].Text)
	require.Equal(t, reason, body[2].Facts[5].Value)
}

func (msg testTeamsMessage) checkStatusDenied(t *testing.T, reason string) {
	t.Helper()
	body := msg.Attachments[0].Content.Body
	require.GreaterOrEqual(t, len(body), 3)
	require.Equal(t, "❌", body[1].Columns[0].Items[0].Text)
	require.Equal(t, "DENIED", body[1].Columns[1].Items[0].Text)
	require.Equal(t, reason, body[2].Facts[5].Value)
}

func (msg testTeamsMessage) checkStatusExpired(t *testing.T) {
	t.Helper()
	body := msg.Attachments[0].Content.Body
	require.GreaterOrEqual(t, len(body), 3)
	require.Equal(t, "⌛", body[1].Columns[0].Items[0].Text)
	require.Equal(t, "EXPIRED", body[1].Columns[1].Items[0].Text)
}

func (msg testTeamsMessage) checkReview(t *testing.T, index int, approved bool, reason, reviewer string) {
	t.Helper()
	body := msg.Attachments[0].Content.Body
	require.GreaterOrEqual(t, len(body), 4+index)
	part := body[4+index]
	switch approved {
	case true:
		require.Equal(t, "✅", part.Facts[0].Value)
	case false:
		require.Equal(t, "❌", part.Facts[0].Value)
	}
	require.Equal(t, reviewer, part.Facts[1].Value)
	require.Equal(t, reason, part.Facts[3].Value)
}

func (msg testTeamsMessage) getTitle() string {
	return msg.Attachments[0].Content.Body[0].Text
}
