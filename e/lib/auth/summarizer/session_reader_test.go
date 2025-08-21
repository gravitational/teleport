package summarizer

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
)

func TestSessionReader_Success(t *testing.T) {
	t.Parallel()
	type FragmentSpec struct {
		requestLength int
		expected      string
		expectedErr   error
	}
	cases := []struct {
		name              string
		printData         []string
		expectedFragments []FragmentSpec
	}{
		{
			name:      "empty session",
			printData: []string{},
			expectedFragments: []FragmentSpec{{
				requestLength: 1,
				expectedErr:   io.EOF,
			}},
		},
		{
			name:      "fragment size equals total data length",
			printData: []string{"print1", "print2"},
			expectedFragments: []FragmentSpec{{
				requestLength: len("print1print2"),
				expected:      "print1print2",
			}, {
				requestLength: 1,
				expectedErr:   io.EOF,
			}},
		},
		{
			name:      "fragment size larger than total data length",
			printData: []string{"asdf", "qwer"},
			expectedFragments: []FragmentSpec{{
				requestLength: 20,
				expected:      "asdfqwer",
				expectedErr:   io.EOF,
			}},
		},
		{
			name:      "fragment sizes align with print data",
			printData: []string{"print1", "print2"},
			expectedFragments: []FragmentSpec{{
				requestLength: len("print1"),
				expected:      "print1",
			}, {
				requestLength: len("print2"),
				expected:      "print2",
			}, {
				requestLength: len("print2"),
				expectedErr:   io.EOF,
			}},
		},
		{
			name:      "fragment sizes don't align with print data",
			printData: []string{"print1", "print2"},
			expectedFragments: []FragmentSpec{{
				requestLength: 4,
				expected:      "prin",
			}, {
				requestLength: 5,
				expected:      "t1pri",
			}, {
				requestLength: 8,
				expected:      "nt2",
				expectedErr:   io.EOF,
			}},
		},
		{
			name:      "reading past end of data",
			printData: []string{"foobar"},
			expectedFragments: []FragmentSpec{{
				requestLength: 20,
				expected:      "foobar",
				expectedErr:   io.EOF,
			}, {
				requestLength: 20,
				expectedErr:   io.EOF,
			}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			t.Cleanup(cancel)

			sid := "73354326-b9c5-4a05-b3c2-6fc024c5026f"
			evts := eventstest.GenerateTestSession(eventstest.SessionParams{
				SessionID: sid,
				PrintData: tc.printData,
			})
			streamer := &eventstest.MockAuditLog{SessionEvents: evts}
			r := newSessionReader(ctx, streamer, session.ID(sid))

			for _, fragment := range tc.expectedFragments {
				buf := make([]byte, fragment.requestLength)
				n, err := r.Read(buf)
				assert.Equal(t, fragment.expected, string(buf[:n]))
				if fragment.expectedErr == nil {
					assert.NoError(t, err)
				} else {
					assert.ErrorIs(t, err, fragment.expectedErr)
				}
			}
		})
		// TODO make sure that the channel doesn't expect to be closed.
	}
}

type failingStreamer struct {
}

func (r failingStreamer) StreamSessionEvents(
	ctx context.Context, sessionID session.ID, startIndex int64,
) (chan apievents.AuditEvent, chan error) {
	eventsCh := make(chan apievents.AuditEvent)
	// Make channel buffered to avoid blocking.
	errCh := make(chan error, 1)
	errCh <- trace.NotFound("oh noes")
	return eventsCh, errCh
}

func TestSessionReader_Error(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	r := newSessionReader(ctx, failingStreamer{}, "1715bdbd-910f-4615-96cb-55c549e0fa99")

	n, err := r.Read(make([]byte, 1))
	assert.Equal(t, 0, n)
	// Important to check the exact error, as the reader may also return "context
	// deadline exceeded".
	assert.ErrorIs(t, err, trace.NotFound("oh noes"))
}
