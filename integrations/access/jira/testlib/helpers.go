package testlib

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gravitational/teleport/integrations/access/jira"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"net/http"
)

func (s *JiraSuite) checkPluginData(ctx context.Context, reqID string, cond func(jira.PluginData) bool) jira.PluginData {
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

func (s *JiraSuite) postWebhook(ctx context.Context, url, issueID, status string) (*http.Response, error) {
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

func (s *JiraSuite) postWebhookAndCheck(ctx context.Context, url, issueID, status string) {
	t := s.T()
	t.Helper()

	resp, err := s.postWebhook(ctx, url, issueID, status)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
