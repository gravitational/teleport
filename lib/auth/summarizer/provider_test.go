// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package summarizer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

func TestProvideSummarizer(t *testing.T) {
	var provider *SessionSummarizerProvider
	assert.IsType(t, &NoopSummarizer{}, provider.SessionSummarizer(),
		"nil provider should return a noop summarizer")

	provider = NewSessionSummarizerProvider()
	assert.IsType(t, &NoopSummarizer{}, provider.SessionSummarizer(),
		"new provider should return a noop summarizer")

	s := &dummySummarizer{}
	provider.SetSummarizer(s)
	assert.Equal(t, s, provider.SessionSummarizer(), "should return the set summarizer")

	provider.SetSummarizer(nil)
	assert.IsType(t, &NoopSummarizer{}, provider.SessionSummarizer(),
		"after setting a nil summarizer, the provider should return a noop one instead")

	var ss SessionSummarizer
	var ds *dummySummarizer = nil
	ss = ds
	provider.SetSummarizer(ss)
	assert.IsType(t, &NoopSummarizer{}, provider.SessionSummarizer(),
		"after setting a nil summarizer wrapped in a non-nil interface pointer, the provider should return a noop one instead")
}

type dummySummarizer struct{}

func (m *dummySummarizer) Summarize(ctx context.Context, sessionID session.ID, sessionEndEvent *events.OneOf) error {
	return nil
}
