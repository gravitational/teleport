package ttyterminal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
)

func TestStreamTtyRecording_BracketedPasteMode(t *testing.T) {
	evtChan, errChan := makeEventChan(
		sessionStart(),
		bracketedPasteOn(100),
		sessionPrint("ls -la", 200),
		bracketedPasteOff(300),
		sessionPrint("output line 1\r\noutput line 2\r\n", 400),
		&apievents.SessionEnd{},
	)

	stream, err := StreamTTYRecording(t.Context(), evtChan, errChan)
	require.NoError(t, err)
	require.True(t, stream.HasCommands())
	require.Len(t, collectCommandFromStream(stream), 1)
	require.NoError(t, stream.Wait())
}

func TestStreamTtyRecording_TokenStreamMode(t *testing.T) {
	evtChan, errChan := makeEventChan(
		sessionStart(),
		sessionPrint("simple text output", 100),
		sessionPrint("more output", 200),
		&apievents.SessionEnd{},
	)

	stream, err := StreamTTYRecording(t.Context(), evtChan, errChan)
	require.NoError(t, err)
	require.False(t, stream.HasCommands())
}

func TestStreamTtyRecording_EmptyStream(t *testing.T) {
	evtChan, errChan := makeEventChan(sessionStart(), &apievents.SessionEnd{})

	stream, err := StreamTTYRecording(t.Context(), evtChan, errChan)
	require.NoError(t, err)
	require.False(t, stream.HasCommands())
}

func TestStreamTtyRecording_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	evtChan := make(chan apievents.AuditEvent, 10)
	evtChan <- sessionStart()
	cancel()

	_, err := StreamTTYRecording(ctx, evtChan, make(chan error, 1))
	require.ErrorIs(t, err, context.Canceled)
}

func TestStreamTtyRecording_MultipleCommands(t *testing.T) {
	evtChan, errChan := makeEventChan(
		sessionStart(),
		bracketedPasteOn(100),
		sessionPrint("cmd1", 150),
		bracketedPasteOff(200),
		sessionPrint("output1\r\n", 250),
		bracketedPasteOn(300),
		sessionPrint("cmd2", 350),
		bracketedPasteOff(400),
		sessionPrint("output2\r\n", 450),
		&apievents.SessionEnd{},
	)

	stream, err := StreamTTYRecording(t.Context(), evtChan, errChan)
	require.NoError(t, err)
	require.True(t, stream.HasCommands())
	require.Len(t, collectCommandFromStream(stream), 2)
	require.NoError(t, stream.Wait())
}

func TestStreamTtyRecording_WithResize(t *testing.T) {
	startTime := time.Now()
	evtChan, errChan := makeEventChan(
		sessionStart(),
		bracketedPasteOn(100),
		&apievents.Resize{
			Metadata:     apievents.Metadata{Time: startTime.Add(150 * time.Millisecond)},
			TerminalSize: "100:50",
		},
		sessionPrint("command", 200),
		bracketedPasteOff(250),
		&apievents.SessionEnd{},
	)

	stream, err := StreamTTYRecording(t.Context(), evtChan, errChan)
	require.NoError(t, err)
	require.True(t, stream.HasCommands())
	require.Len(t, collectCommandFromStream(stream), 1)
	require.NoError(t, stream.Wait())
}

func TestPeekTokens(t *testing.T) {
	t.Parallel()

	makeTokenChan := func(tokens ...token) chan token {
		ch := make(chan token, len(tokens))
		for _, tok := range tokens {
			ch <- tok
		}
		close(ch)
		return ch
	}

	collectTokens := func(ch <-chan token) []token {
		var tokens []token
		for tok := range ch {
			tokens = append(tokens, tok)
		}
		return tokens
	}

	tests := []struct {
		name           string
		tokenCount     int
		maxPeek        int
		expectedPeeked int
		expectedTotal  int
	}{
		{"LessThanMax", 2, 10, 2, 2},
		{"ExactlyMax", 5, 5, 5, 5},
		{"MoreThanMax", 15, 5, 5, 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tokens []token
			for i := 0; i < tt.tokenCount; i++ {
				tokens = append(tokens, token{tokenType: tokenText, data: []byte("token")})
			}

			peeked, replayed, err := peekTokens(t.Context(), makeTokenChan(tokens...), tt.maxPeek)
			require.NoError(t, err)
			require.Len(t, peeked, tt.expectedPeeked)
			require.Len(t, collectTokens(replayed), tt.expectedTotal)
		})
	}

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, _, err := peekTokens(ctx, make(chan token, 10), 10)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestReplayTokens(t *testing.T) {
	t.Parallel()

	collectTokens := func(ch <-chan token) []token {
		var tokens []token
		for tok := range ch {
			tokens = append(tokens, tok)
		}
		return tokens
	}

	t.Run("PreservesOrder", func(t *testing.T) {
		peeked := []token{
			{tokenType: tokenText, data: []byte("first"), timestamp: 100 * time.Millisecond},
			{tokenType: tokenResize, data: []byte("80:24"), timestamp: 200 * time.Millisecond},
			{tokenType: tokenText, data: []byte("third"), timestamp: 300 * time.Millisecond},
		}

		remainingChan := make(chan token, 2)
		remainingChan <- token{tokenType: tokenText, data: []byte("fourth"), timestamp: 400 * time.Millisecond}
		remainingChan <- token{tokenType: tokenText, data: []byte("fifth"), timestamp: 500 * time.Millisecond}
		close(remainingChan)

		tokens := collectTokens(replayTokens(t.Context(), peeked, remainingChan))

		require.Len(t, tokens, 5)
		expected := []string{"first", "80:24", "third", "fourth", "fifth"}
		for i, exp := range expected {
			require.Equal(t, exp, string(tokens[i].data))
		}
	})

	t.Run("Empty", func(t *testing.T) {
		remainingChan := make(chan token, 1)
		remainingChan <- token{tokenType: tokenText, data: []byte("only")}
		close(remainingChan)

		tokens := collectTokens(replayTokens(t.Context(), nil, remainingChan))

		require.Len(t, tokens, 1)
		require.Equal(t, "only", string(tokens[0].data))
	})
}

func makeEventChan(events ...apievents.AuditEvent) (chan apievents.AuditEvent, chan error) {
	evtChan := make(chan apievents.AuditEvent, len(events))
	for _, e := range events {
		evtChan <- e
	}
	close(evtChan)
	return evtChan, make(chan error, 1)
}

func sessionStart() *apievents.SessionStart {
	return &apievents.SessionStart{TerminalSize: "80:24"}
}

func sessionPrint(data string, delayMs int64) *apievents.SessionPrint {
	return &apievents.SessionPrint{Data: []byte(data), DelayMilliseconds: delayMs}
}

func bracketedPasteOn(delayMs int64) *apievents.SessionPrint {
	return sessionPrint("\x1b[?2004h", delayMs)
}

func bracketedPasteOff(delayMs int64) *apievents.SessionPrint {
	return sessionPrint("\x1b[?2004l", delayMs)
}

func collectCommandFromStream(stream *TTYRecordingStream) []Command {
	var commands []Command
	for cmd := range stream.Commands() {
		commands = append(commands, cmd)
	}
	return commands
}
