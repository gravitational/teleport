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
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/access/jira"
)

func (s *JiraBaseSuite) checkPluginData(ctx context.Context, reqID string, cond func(jira.PluginData) bool) jira.PluginData {
	t := s.T()
	t.Helper()

	for {
		rawData, err := s.Ruler().PollAccessRequestPluginData(ctx, "jira", reqID)
		require.NoError(t, err)
		if data := jira.DecodePluginData(rawData); cond(data) {
			return data
		}
	}
}

func (s *JiraBaseSuite) postWebhook(ctx context.Context, url, issueID, status string) (*http.Response, error) {
	var buf bytes.Buffer
	wh := jira.Webhook{
		WebhookEvent:       "jira:issue_updated",
		IssueEventTypeName: "issue_generic",
		Issue: &jira.WebhookIssue{
			ID: issueID,
			Fields: jira.IssueFields{
				Status: jira.StatusDetails{
					Name: status,
				},
			},
		},
	}
	err := json.NewEncoder(&buf).Encode(&wh)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	request.Header.Add("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	return response, trace.Wrap(err)
}

func (s *JiraBaseSuite) postWebhookAndCheck(ctx context.Context, url, issueID, status string) {
	s.T().Helper()
	t := s.T()

	resp, err := s.postWebhook(ctx, url, issueID, status)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
