package ttyterminal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name           string
		events         []apievents.AuditEvent
		expectedTokens []token
		description    string
	}{
		{
			name: "basic_text_only",
			events: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:     apievents.Metadata{Time: time.Now()},
					TerminalSize: "80:24",
				},
				&apievents.SessionPrint{
					Data:              []byte("Hello, World!"),
					DelayMilliseconds: 100,
				},
				&apievents.SessionEnd{},
			},
			expectedTokens: []token{
				{tokenType: tokenResize, data: []byte("80:24")},
				{tokenType: tokenText, data: []byte("Hello, World!")},
			},
			description: "Simple text without any escape sequences",
		},
		{
			name: "bracketed_paste_mode",
			events: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:     apievents.Metadata{Time: time.Now()},
					TerminalSize: "80:24",
				},
				&apievents.SessionPrint{
					Data:              []byte("normal text\x1b[?2004hpasted content\x1b[?2004lmore text"),
					DelayMilliseconds: 200,
				},
				&apievents.SessionEnd{},
			},
			expectedTokens: []token{
				{tokenType: tokenResize, data: []byte("80:24")},
				{tokenType: tokenText, data: []byte("normal text")},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h")},
				{tokenType: tokenText, data: []byte("pasted content")},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l")},
				{tokenType: tokenText, data: []byte("more text")},
			},
			description: "Text with bracketed paste mode sequences",
		},
		{
			name: "alternate_screen_transitions",
			events: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:     apievents.Metadata{Time: time.Now()},
					TerminalSize: "80:24",
				},
				&apievents.SessionPrint{
					Data:              []byte("before vim\x1b[?1049husing vim\x1b[?1049lafter vim"),
					DelayMilliseconds: 300,
				},
				&apievents.SessionEnd{},
			},
			expectedTokens: []token{
				{tokenType: tokenResize, data: []byte("80:24")},
				{tokenType: tokenText, data: []byte("before vim")},
				{tokenType: tokenAlternateScreenEnter, data: []byte("\x1b[?1049h")},
				{tokenType: tokenText, data: []byte("using vim")},
				{tokenType: tokenAlternateScreenExit, data: []byte("\x1b[?1049l")},
				{tokenType: tokenText, data: []byte("after vim")},
			},
			description: "Alternate screen buffer transitions (like entering/exiting vim)",
		},
		{
			name: "multiple_escape_sequences",
			events: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:     apievents.Metadata{Time: time.Now()},
					TerminalSize: "80:24",
				},
				&apievents.SessionPrint{
					Data:              []byte("\x1b[?2004h\x1b[?1049h\x1b[?2004l\x1b[?1049l"),
					DelayMilliseconds: 400,
				},
				&apievents.SessionEnd{},
			},
			expectedTokens: []token{
				{tokenType: tokenResize, data: []byte("80:24")},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h")},
				{tokenType: tokenAlternateScreenEnter, data: []byte("\x1b[?1049h")},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l")},
				{tokenType: tokenAlternateScreenExit, data: []byte("\x1b[?1049l")},
			},
			description: "Multiple consecutive escape sequences without text",
		},
		{
			name: "mixed_content_complex",
			events: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:     apievents.Metadata{Time: time.Now()},
					TerminalSize: "120:40",
				},
				&apievents.SessionPrint{
					Data:              []byte("$ ls -la\x1b[?2004hfilename.txt\x1b[?2004l\n"),
					DelayMilliseconds: 100,
				},
				&apievents.SessionPrint{
					Data:              []byte("$ vim file.txt\x1b[?1049h"),
					DelayMilliseconds: 200,
				},
				&apievents.SessionPrint{
					Data:              []byte("editing content"),
					DelayMilliseconds: 300,
				},
				&apievents.SessionPrint{
					Data:              []byte("\x1b[?1049l:wq\n"),
					DelayMilliseconds: 400,
				},
				&apievents.SessionEnd{},
			},
			expectedTokens: []token{
				{tokenType: tokenResize, data: []byte("120:40")},
				{tokenType: tokenText, data: []byte("$ ls -la")},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h")},
				{tokenType: tokenText, data: []byte("filename.txt")},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l")},
				{tokenType: tokenText, data: []byte("\n")},
				{tokenType: tokenText, data: []byte("$ vim file.txt")},
				{tokenType: tokenAlternateScreenEnter, data: []byte("\x1b[?1049h")},
				{tokenType: tokenText, data: []byte("editing content")},
				{tokenType: tokenAlternateScreenExit, data: []byte("\x1b[?1049l")},
				{tokenType: tokenText, data: []byte(":wq\n")},
			},
			description: "Complex scenario with multiple events and mixed content",
		},
		{
			name: "terminal_resize_events",
			events: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:     apievents.Metadata{Time: time.Now()},
					TerminalSize: "80:24",
				},
				&apievents.SessionPrint{
					Data:              []byte("initial text"),
					DelayMilliseconds: 100,
				},
				&apievents.Resize{
					Metadata:     apievents.Metadata{Time: time.Now().Add(200 * time.Millisecond)},
					TerminalSize: "120:40",
				},
				&apievents.SessionPrint{
					Data:              []byte("after resize"),
					DelayMilliseconds: 300,
				},
				&apievents.SessionEnd{},
			},
			expectedTokens: []token{
				{tokenType: tokenResize, data: []byte("80:24")},
				{tokenType: tokenText, data: []byte("initial text")},
				{tokenType: tokenResize, data: []byte("120:40")},
				{tokenType: tokenText, data: []byte("after resize")},
			},
			description: "Terminal resize during session",
		},
		{
			name: "incomplete_escape_sequence",
			events: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:     apievents.Metadata{Time: time.Now()},
					TerminalSize: "80:24",
				},
				&apievents.SessionPrint{
					Data:              []byte("text\x1b[?200not a complete sequence"),
					DelayMilliseconds: 100,
				},
				&apievents.SessionEnd{},
			},
			expectedTokens: []token{
				{tokenType: tokenResize, data: []byte("80:24")},
				{tokenType: tokenText, data: []byte("text\x1b[?200not a complete sequence")},
			},
			description: "Incomplete escape sequence should be treated as regular text",
		},
		{
			name: "alternative_screen_modes",
			events: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:     apievents.Metadata{Time: time.Now()},
					TerminalSize: "80:24",
				},
				&apievents.SessionPrint{
					Data:              []byte("test\x1b[?1047hmode1047\x1b[?1047l"),
					DelayMilliseconds: 100,
				},
				&apievents.SessionPrint{
					Data:              []byte("\x1b[?47hmode47\x1b[?47l"),
					DelayMilliseconds: 200,
				},
				&apievents.SessionEnd{},
			},
			expectedTokens: []token{
				{tokenType: tokenResize, data: []byte("80:24")},
				{tokenType: tokenText, data: []byte("test")},
				{tokenType: tokenAlternateScreenEnter, data: []byte("\x1b[?1047h")},
				{tokenType: tokenText, data: []byte("mode1047")},
				{tokenType: tokenAlternateScreenExit, data: []byte("\x1b[?1047l")},
				{tokenType: tokenAlternateScreenEnter, data: []byte("\x1b[?47h")},
				{tokenType: tokenText, data: []byte("mode47")},
				{tokenType: tokenAlternateScreenExit, data: []byte("\x1b[?47l")},
			},
			description: "Different alternate screen mode sequences",
		},
		{
			name: "escape_in_middle_of_text",
			events: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:     apievents.Metadata{Time: time.Now()},
					TerminalSize: "80:24",
				},
				&apievents.SessionPrint{
					Data:              []byte("before\x1b[?2004hmiddle\x1b[?2004lafter"),
					DelayMilliseconds: 100,
				},
				&apievents.SessionEnd{},
			},
			expectedTokens: []token{
				{tokenType: tokenResize, data: []byte("80:24")},
				{tokenType: tokenText, data: []byte("before")},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h")},
				{tokenType: tokenText, data: []byte("middle")},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l")},
				{tokenType: tokenText, data: []byte("after")},
			},
			description: "Escape sequences appearing in the middle of text",
		},
		{
			name: "empty_events",
			events: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:     apievents.Metadata{Time: time.Now()},
					TerminalSize: "80:24",
				},
				&apievents.SessionPrint{
					Data:              []byte(""),
					DelayMilliseconds: 100,
				},
				&apievents.SessionPrint{
					Data:              []byte("not empty"),
					DelayMilliseconds: 200,
				},
				&apievents.SessionEnd{},
			},
			expectedTokens: []token{
				{tokenType: tokenResize, data: []byte("80:24")},
				{tokenType: tokenText, data: []byte("not empty")},
			},
			description: "Empty print events should not generate tokens",
		},
		{
			name: "unicode_and_special_chars",
			events: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:     apievents.Metadata{Time: time.Now()},
					TerminalSize: "80:24",
				},
				&apievents.SessionPrint{
					Data:              []byte("こんにちは 🚀 emoji\x1b[?2004h世界\x1b[?2004l"),
					DelayMilliseconds: 100,
				},
				&apievents.SessionEnd{},
			},
			expectedTokens: []token{
				{tokenType: tokenResize, data: []byte("80:24")},
				{tokenType: tokenText, data: []byte("こんにちは 🚀 emoji")},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h")},
				{tokenType: tokenText, data: []byte("世界")},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l")},
			},
			description: "Unicode characters and emojis with escape sequences",
		},
		{
			name: "adjacent_escape_sequences",
			events: []apievents.AuditEvent{
				&apievents.SessionStart{
					Metadata:     apievents.Metadata{Time: time.Now()},
					TerminalSize: "80:24",
				},
				&apievents.SessionPrint{
					Data:              []byte("\x1b[?2004h\x1b[?2004l\x1b[?1049h\x1b[?1049l"),
					DelayMilliseconds: 100,
				},
				&apievents.SessionEnd{},
			},
			expectedTokens: []token{
				{tokenType: tokenResize, data: []byte("80:24")},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h")},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l")},
				{tokenType: tokenAlternateScreenEnter, data: []byte("\x1b[?1049h")},
				{tokenType: tokenAlternateScreenExit, data: []byte("\x1b[?1049l")},
			},
			description: "Adjacent escape sequences without intervening text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			tokenizer := newTokenizer()

			eventChan, errChan := createEventsStream(ctx, tt.events)

			processDone := make(chan error, 1)
			go func() {
				processDone <- tokenizer.processEventStream(ctx, eventChan, errChan)
			}()

			tokens := collectTokens(t, tokenizer, processDone, 2*time.Second)

			require.Len(t, tt.expectedTokens, len(tokens),
				"Scenario: %s - Expected %d tokens but got %d",
				tt.description, len(tt.expectedTokens), len(tokens))

			for i, expected := range tt.expectedTokens {
				if i >= len(tokens) {
					break
				}

				actual := tokens[i]

				require.Equal(t, expected.tokenType, actual.tokenType,
					"Token %d type mismatch in scenario: %s", i, tt.description)
				require.Equal(t, expected.data, actual.data,
					"Token %d data mismatch in scenario: %s", i, tt.description)
			}
		})
	}
}

func TestTokenizerErrorHandling(t *testing.T) {
	t.Run("error_from_channel", func(t *testing.T) {
		ctx := t.Context()
		eventChan := make(chan apievents.AuditEvent)
		errChan := make(chan error, 1)

		tokenizer := newTokenizer()

		testErr := errors.New("test error")
		errChan <- testErr

		err := tokenizer.processEventStream(ctx, eventChan, errChan)
		require.Error(t, err)
		require.Contains(t, err.Error(), "test error")
	})

	t.Run("context_cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		eventChan := make(chan apievents.AuditEvent)
		errChan := make(chan error)

		tokenizer := newTokenizer()

		processDone := make(chan error)
		go func() {
			processDone <- tokenizer.processEventStream(ctx, eventChan, errChan)
		}()

		cancel()

		select {
		case err := <-processDone:
			require.Error(t, err)
			require.Equal(t, context.Canceled, err)
		case <-time.After(1 * time.Second):
			t.Fatal("Test timed out waiting for context cancellation")
		}
	})
}

func TestTokenizerTimestamps(t *testing.T) {
	ctx := t.Context()
	tokenizer := newTokenizer()

	startTime := time.Now()

	events := []apievents.AuditEvent{
		&apievents.SessionStart{
			Metadata:     apievents.Metadata{Time: startTime},
			TerminalSize: "80:24",
		},
		&apievents.SessionPrint{
			Data:              []byte("first"),
			DelayMilliseconds: 100,
		},
		&apievents.SessionPrint{
			Data:              []byte("second"),
			DelayMilliseconds: 200,
		},
		&apievents.Resize{
			Metadata:     apievents.Metadata{Time: startTime.Add(300 * time.Millisecond)},
			TerminalSize: "120:40",
		},
		&apievents.SessionEnd{},
	}

	eventChan, errChan := createEventsStream(ctx, events)

	processDone := make(chan error)
	go func() {
		processDone <- tokenizer.processEventStream(ctx, eventChan, errChan)
	}()

	tokens := collectTokens(t, tokenizer, processDone, 1*time.Second)

	require.Len(t, tokens, 4, "Should have 4 tokens")

	require.Equal(t, time.Duration(0), tokens[0].timestamp, "Initial resize should have 0 timestamp")
	require.Equal(t, 100*time.Millisecond, tokens[1].timestamp, "First print should have 100ms delay")
	require.Equal(t, 200*time.Millisecond, tokens[2].timestamp, "Second print should have 200ms delay")
	require.Equal(t, 300*time.Millisecond, tokens[3].timestamp, "Resize should have 300ms from start")
}

func collectTokens(t *testing.T, tokenizer *tokenizer, processDone <-chan error, timeout time.Duration) []token {
	timeoutChan := time.After(timeout)

	select {
	case err := <-processDone: // wait for processing to complete or error
		if err != nil {
			t.Fatalf("Processing error: %v", err)
		}
	case <-timeoutChan:
		t.Fatal("Test timed out waiting for processing")
	}

	var tokens []token
	for tok := range tokenizer.tokens() {
		tokens = append(tokens, tok)
	}

	return tokens
}

func createEventsStream(ctx context.Context, events []apievents.AuditEvent) (chan apievents.AuditEvent, chan error) {
	errors := make(chan error, 1)
	evts := make(chan apievents.AuditEvent)

	go func() {
		defer close(evts)
		defer close(errors)

		for _, evt := range events {
			select {
			case <-ctx.Done():
				return
			case evts <- evt:
			}
		}
	}()

	return evts, errors
}
