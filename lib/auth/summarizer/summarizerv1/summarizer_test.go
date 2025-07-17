package summarizerv1

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/stretchr/testify/assert"
)

func TestProvideSummarizer(t *testing.T) {
	var provider *SummarizerProvider
	assert.Nil(t, provider.ProvideSummarizer(), "nil provider should return nil summarizer")

	provider = NewSummarizerProvider()
	assert.Nil(t, provider.ProvideSummarizer(), "new provider should return nil summarizer")

	s := &dummySummarizer{}
	provider.SetSummarizer(s)
	assert.Equal(t, s, provider.ProvideSummarizer(), "should return the set summarizer")
}

type dummySummarizer struct{}

func (m *dummySummarizer) Summarize(ctx context.Context, sessionID session.ID, sessionEndEvent *events.OneOf) error {
	return nil
}
