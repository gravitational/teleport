package ttyterminal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

func TestProperty_StreamTTYRecording_NeverPanicsOnRandomEvents(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(0, 20).Draw(t, "count")
		events := make([]apievents.AuditEvent, 0, count+1)
		for i := 0; i < count; i++ {
			events = append(events, genStreamEvent(t, "evt"))
		}
		events = append(events, &apievents.SessionEnd{})

		testutils.RunWithTimeout(t, 3*time.Second, func() {
			evtChan, errChan := makeEventChan(events...)
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			stream, err := StreamTTYRecording(ctx, evtChan, errChan, fakeTokenCounter{})
			if err != nil {
				return
			}

			// Drain commands, then Wait. A worker panic becomes errInternalProcessing, so assert Wait
			// never returns it — otherwise a recovered panic would be silently swallowed here. Other
			// errors are expected on random input and ignored.
			if stream.HasCommands() {
				for range stream.Commands() {
				}
			}
			require.NotErrorIs(t, stream.Wait(), errInternalProcessing)
		})
	})
}

func TestProperty_StreamTTYRecording_CancelMidStreamShutsDown(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(1, 10).Draw(t, "count")
		events := []apievents.AuditEvent{
			&apievents.SessionStart{TerminalSize: "80:24"},
			&apievents.SessionPrint{Data: []byte("\x1b[?2004h")},
		}
		for i := 0; i < count; i++ {
			events = append(events, &apievents.SessionPrint{Data: []byte("hello\r\n")})
		}
		events = append(events, &apievents.SessionPrint{Data: []byte("\x1b[?2004l")})

		testutils.RunWithTimeout(t, 3*time.Second, func() {
			evtChan, errChan := makeEventChan(events...)
			ctx, cancel := context.WithCancel(t.Context())

			stream, err := StreamTTYRecording(ctx, evtChan, errChan, fakeTokenCounter{})
			require.NoError(t, err)

			cancel()

			if stream.HasCommands() {
				for range stream.Commands() {
				}
			}

			if err := stream.Wait(); err != nil {
				require.ErrorIs(t, err, context.Canceled,
					"shutdown after cancel surfaced a non-cancel error: %v", err)
			}
		})
	})
}

func TestStreamTTYRecording_PropagatesStreamError(t *testing.T) {
	sentinel := errors.New("stream boom")
	evtChan := make(chan apievents.AuditEvent, 32)
	errChan := make(chan error, 1)

	evtChan <- &apievents.SessionStart{TerminalSize: "80:24"}
	evtChan <- &apievents.SessionPrint{Data: []byte("\x1b[?2004h")}
	for i := 0; i < 12; i++ {
		evtChan <- &apievents.SessionPrint{Data: []byte("\x1b[1mx")}
	}

	var waitErr error
	testutils.RunWithTimeout(t, 3*time.Second, func() {
		stream, err := StreamTTYRecording(t.Context(), evtChan, errChan, fakeTokenCounter{})
		require.NoError(t, err)
		require.True(t, stream.HasCommands())

		errChan <- sentinel

		for range stream.Commands() {
		}
		waitErr = stream.Wait()
	})

	require.ErrorIs(t, waitErr, sentinel)
}

func TestStreamTTYRecording_PeekPhaseSurfacesRealError(t *testing.T) {
	sentinel := errors.New("read boom")
	evtChan := make(chan apievents.AuditEvent, 2)
	errChan := make(chan error, 1)

	evtChan <- &apievents.SessionStart{TerminalSize: "80:24"}
	errChan <- sentinel

	var got error
	testutils.RunWithTimeout(t, 3*time.Second, func() {
		_, got = StreamTTYRecording(t.Context(), evtChan, errChan, fakeTokenCounter{})
	})

	require.ErrorIs(t, got, sentinel)
}

// genStreamEvent generates a random AuditEvent suitable for the TTY pipeline.
// Bias toward sequences that exercise the bracketed-paste / alternate-screen state.
func genStreamEvent(t *rapid.T, label string) apievents.AuditEvent {
	t.Helper()

	kind := rapid.SampledFrom([]string{
		"start", "print", "print_paste_on", "print_paste_off", "print_alt_on", "print_alt_off",
		"print_random", "resize", "end",
	}).Draw(t, label+"_kind")

	delay := rapid.Int64Range(0, 10_000).Draw(t, label+"_delay")

	switch kind {
	case "start":
		return &apievents.SessionStart{TerminalSize: "80:24"}

	case "print":
		return &apievents.SessionPrint{
			Data:              []byte("hello\r\n"),
			DelayMilliseconds: delay,
		}

	case "print_paste_on":
		return &apievents.SessionPrint{
			Data:              []byte("\x1b[?2004h"),
			DelayMilliseconds: delay,
		}

	case "print_paste_off":
		return &apievents.SessionPrint{
			Data:              []byte("\x1b[?2004l"),
			DelayMilliseconds: delay,
		}

	case "print_alt_on":
		return &apievents.SessionPrint{
			Data:              []byte("\x1b[?1049h"),
			DelayMilliseconds: delay,
		}

	case "print_alt_off":
		return &apievents.SessionPrint{
			Data:              []byte("\x1b[?1049l"),
			DelayMilliseconds: delay,
		}

	case "print_random":
		return &apievents.SessionPrint{
			Data:              genAdversarialBytes(t, label+"_bytes", 128),
			DelayMilliseconds: delay,
		}

	case "resize":
		return &apievents.Resize{TerminalSize: "100:30"}

	case "end":
		return &apievents.SessionEnd{}
	}

	return &apievents.SessionEnd{}
}
