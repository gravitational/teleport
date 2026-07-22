/*
Copyright 2024 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testlib

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/access/email"
)

func (s *EmailBaseSuite) checkPluginData(ctx context.Context, reqID string, cond func(email.PluginData) bool) email.PluginData {
	t := s.T()
	t.Helper()

	for {
		rawData, err := s.Ruler().PollAccessRequestPluginData(ctx, "email", reqID)
		require.NoError(t, err)
		if data := email.DecodePluginData(rawData); cond(data) {
			return data
		}
	}
}

// skipEmails ensures that emails were received, but dumps the contents
func (s *EmailBaseSuite) skipMessages(ctx context.Context, t *testing.T, n int) {
	for range n {
		_, err := s.mockMailgun.GetMessage(ctx)
		require.NoError(t, err)
	}
}

// getMessages returns next n email messages
func (s *EmailBaseSuite) getMessages(ctx context.Context, t *testing.T, n int) []mockMailgunMessage {
	messages := make([]mockMailgunMessage, n)
	for i := range n {
		m, err := s.mockMailgun.GetMessage(ctx)
		require.NoError(t, err)
		messages[i] = m
	}

	return messages
}

// extractRequestID extracts request id from a subject
func (s *EmailBaseSuite) extractRequestID(subject string) string {
	idx := strings.Index(subject, subjectIDSubstring)
	return subject[idx+len(subjectIDSubstring):]
}
