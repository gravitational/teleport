package ttyterminal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParser(t *testing.T) {
	tests := []struct {
		name             string
		tokens           []token
		expectedCommands []*command
		description      string
	}{
		{
			name: "basic_command_with_output",
			tokens: []token{
				{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
				{tokenType: tokenText, data: []byte("ls -la"), timestamp: 200 * time.Millisecond},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 300 * time.Millisecond},
				{tokenType: tokenText, data: []byte("file1.txt\nfile2.txt\n"), timestamp: 400 * time.Millisecond},
			},
			expectedCommands: []*command{
				{
					startTime: 100 * time.Millisecond,
					endTime:   400 * time.Millisecond,
					input: commandData{
						startTime: 100 * time.Millisecond,
						endTime:   300 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("ls -la"), timestamp: 200 * time.Millisecond},
						},
					},
					output: commandData{
						startTime: 300 * time.Millisecond,
						endTime:   400 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("file1.txt\nfile2.txt\n"), timestamp: 400 * time.Millisecond},
						},
					},
				},
			},
			description: "Simple command with bracketed paste and output",
		},
		{
			name: "multiple_commands",
			tokens: []token{
				{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
				// First command
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
				{tokenType: tokenText, data: []byte("echo hello"), timestamp: 200 * time.Millisecond},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 300 * time.Millisecond},
				{tokenType: tokenText, data: []byte("hello\n"), timestamp: 400 * time.Millisecond},
				// Second command
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 500 * time.Millisecond},
				{tokenType: tokenText, data: []byte("pwd"), timestamp: 600 * time.Millisecond},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 700 * time.Millisecond},
				{tokenType: tokenText, data: []byte("/home/user\n"), timestamp: 800 * time.Millisecond},
			},
			expectedCommands: []*command{
				{
					startTime: 100 * time.Millisecond,
					endTime:   500 * time.Millisecond,
					input: commandData{
						startTime: 100 * time.Millisecond,
						endTime:   300 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("echo hello"), timestamp: 200 * time.Millisecond},
						},
					},
					output: commandData{
						startTime: 300 * time.Millisecond,
						endTime:   500 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("hello\n"), timestamp: 400 * time.Millisecond},
						},
					},
				},
				{
					startTime: 500 * time.Millisecond,
					endTime:   800 * time.Millisecond,
					input: commandData{
						startTime: 500 * time.Millisecond,
						endTime:   700 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("pwd"), timestamp: 600 * time.Millisecond},
						},
					},
					output: commandData{
						startTime: 700 * time.Millisecond,
						endTime:   800 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("/home/user\n"), timestamp: 800 * time.Millisecond},
						},
					},
				},
			},
			description: "Multiple sequential commands with outputs",
		},
		{
			name: "nested_bracket_paste",
			tokens: []token{
				{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
				{tokenType: tokenText, data: []byte("outer text"), timestamp: 200 * time.Millisecond},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 300 * time.Millisecond},
				{tokenType: tokenText, data: []byte("inner text"), timestamp: 400 * time.Millisecond},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 500 * time.Millisecond},
				{tokenType: tokenText, data: []byte("more outer"), timestamp: 600 * time.Millisecond},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 700 * time.Millisecond},
				{tokenType: tokenText, data: []byte("output"), timestamp: 800 * time.Millisecond},
			},
			expectedCommands: []*command{
				{
					startTime: 100 * time.Millisecond,
					endTime:   800 * time.Millisecond,
					input: commandData{
						startTime: 100 * time.Millisecond,
						endTime:   700 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("outer text"), timestamp: 200 * time.Millisecond},
							{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 300 * time.Millisecond},
							{tokenType: tokenText, data: []byte("inner text"), timestamp: 400 * time.Millisecond},
							{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 500 * time.Millisecond},
							{tokenType: tokenText, data: []byte("more outer"), timestamp: 600 * time.Millisecond},
						},
					},
					output: commandData{
						startTime: 700 * time.Millisecond,
						endTime:   800 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("output"), timestamp: 800 * time.Millisecond},
						},
					},
				},
			},
			description: "Nested bracketed paste sequences",
		},
		{
			name: "alternate_screen_mode",
			tokens: []token{
				{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
				{tokenType: tokenText, data: []byte("vim file.txt"), timestamp: 200 * time.Millisecond},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 300 * time.Millisecond},
				{tokenType: tokenAlternateScreenEnter, data: []byte("\x1b[?1049h"), timestamp: 400 * time.Millisecond},
				{tokenType: tokenText, data: []byte("editing in vim"), timestamp: 500 * time.Millisecond},
				// Bracket paste in alternate mode should be treated as text
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 600 * time.Millisecond},
				{tokenType: tokenText, data: []byte("pasted text in vim"), timestamp: 700 * time.Millisecond},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 800 * time.Millisecond},
				{tokenType: tokenAlternateScreenExit, data: []byte("\x1b[?1049l"), timestamp: 900 * time.Millisecond},
			},
			expectedCommands: []*command{
				{
					startTime: 100 * time.Millisecond,
					endTime:   900 * time.Millisecond,
					input: commandData{
						startTime: 100 * time.Millisecond,
						endTime:   300 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("vim file.txt"), timestamp: 200 * time.Millisecond},
						},
					},
					output: commandData{
						startTime:         300 * time.Millisecond,
						endTime:           900 * time.Millisecond,
						startSize:         size{cols: 80, rows: 24},
						isAlternateScreen: true,
						tokens: []token{
							{tokenType: tokenText, data: []byte("editing in vim"), timestamp: 500 * time.Millisecond},
							{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 600 * time.Millisecond},
							{tokenType: tokenText, data: []byte("pasted text in vim"), timestamp: 700 * time.Millisecond},
							{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 800 * time.Millisecond},
						},
					},
				},
			},
			description: "Alternate screen mode with bracketed paste treated as text",
		},
		{
			name: "resize_during_command",
			tokens: []token{
				{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
				{tokenType: tokenText, data: []byte("command"), timestamp: 200 * time.Millisecond},
				{tokenType: tokenResize, data: []byte("120:40"), timestamp: 250 * time.Millisecond},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 300 * time.Millisecond},
				{tokenType: tokenResize, data: []byte("100:30"), timestamp: 350 * time.Millisecond},
				{tokenType: tokenText, data: []byte("output"), timestamp: 400 * time.Millisecond},
			},
			expectedCommands: []*command{
				{
					startTime: 100 * time.Millisecond,
					endTime:   400 * time.Millisecond,
					input: commandData{
						startTime: 100 * time.Millisecond,
						endTime:   300 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("command"), timestamp: 200 * time.Millisecond},
							{tokenType: tokenResize, data: []byte("120:40"), timestamp: 250 * time.Millisecond},
						},
					},
					output: commandData{
						startTime: 300 * time.Millisecond,
						endTime:   400 * time.Millisecond,
						startSize: size{cols: 120, rows: 40},
						tokens: []token{
							{tokenType: tokenResize, data: []byte("100:30"), timestamp: 350 * time.Millisecond},
							{tokenType: tokenText, data: []byte("output"), timestamp: 400 * time.Millisecond},
						},
					},
				},
			},
			description: "Terminal resize events during command execution",
		},
		{
			name: "command_without_output",
			tokens: []token{
				{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
				{tokenType: tokenText, data: []byte("clear"), timestamp: 200 * time.Millisecond},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 300 * time.Millisecond},

				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 400 * time.Millisecond},
				{tokenType: tokenText, data: []byte("ls"), timestamp: 500 * time.Millisecond},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 600 * time.Millisecond},
				{tokenType: tokenText, data: []byte("files\n"), timestamp: 700 * time.Millisecond},
			},
			expectedCommands: []*command{
				{
					startTime: 100 * time.Millisecond,
					endTime:   400 * time.Millisecond,
					input: commandData{
						startTime: 100 * time.Millisecond,
						endTime:   300 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("clear"), timestamp: 200 * time.Millisecond},
						},
					},
					output: commandData{
						startTime: 300 * time.Millisecond,
						endTime:   400 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
					},
				},
				{
					startTime: 400 * time.Millisecond,
					endTime:   700 * time.Millisecond,
					input: commandData{
						startTime: 400 * time.Millisecond,
						endTime:   600 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("ls"), timestamp: 500 * time.Millisecond},
						},
					},
					output: commandData{
						startTime: 600 * time.Millisecond,
						endTime:   700 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("files\n"), timestamp: 700 * time.Millisecond},
						},
					},
				},
			},
			description: "command with no output followed by another command",
		},
		{
			name: "empty_input_command",
			tokens: []token{
				{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
				// No input text
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 200 * time.Millisecond},
				{tokenType: tokenText, data: []byte("output anyway\n"), timestamp: 300 * time.Millisecond},
			},
			expectedCommands: []*command{
				{
					startTime: 100 * time.Millisecond,
					endTime:   300 * time.Millisecond,
					input: commandData{
						startTime: 100 * time.Millisecond,
						endTime:   200 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
					},
					output: commandData{
						startTime: 200 * time.Millisecond,
						endTime:   300 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("output anyway\n"), timestamp: 300 * time.Millisecond},
						},
					},
				},
			},
			description: "command with empty input but has output",
		},
		{
			name: "multiple_text_tokens_in_sequence",
			tokens: []token{
				{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
				{tokenType: tokenText, data: []byte("echo "), timestamp: 200 * time.Millisecond},
				{tokenType: tokenText, data: []byte("hello "), timestamp: 250 * time.Millisecond},
				{tokenType: tokenText, data: []byte("world"), timestamp: 300 * time.Millisecond},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 400 * time.Millisecond},
				{tokenType: tokenText, data: []byte("hello "), timestamp: 500 * time.Millisecond},
				{tokenType: tokenText, data: []byte("world\n"), timestamp: 600 * time.Millisecond},
			},
			expectedCommands: []*command{
				{
					startTime: 100 * time.Millisecond,
					endTime:   600 * time.Millisecond,
					input: commandData{
						startTime: 100 * time.Millisecond,
						endTime:   400 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("echo "), timestamp: 200 * time.Millisecond},
							{tokenType: tokenText, data: []byte("hello "), timestamp: 250 * time.Millisecond},
							{tokenType: tokenText, data: []byte("world"), timestamp: 300 * time.Millisecond},
						},
					},
					output: commandData{
						startTime: 400 * time.Millisecond,
						endTime:   600 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("hello "), timestamp: 500 * time.Millisecond},
							{tokenType: tokenText, data: []byte("world\n"), timestamp: 600 * time.Millisecond},
						},
					},
				},
			},
			description: "Multiple text tokens in both input and output",
		},
		{
			name: "trailing_command_on_close",
			tokens: []token{
				{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
				{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
				{tokenType: tokenText, data: []byte("exit"), timestamp: 200 * time.Millisecond},
				{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 300 * time.Millisecond},
				{tokenType: tokenText, data: []byte("goodbye\n"), timestamp: 400 * time.Millisecond},
				// Stream ends here, parser should emit the command on cleanup
			},
			expectedCommands: []*command{
				{
					startTime: 100 * time.Millisecond,
					endTime:   400 * time.Millisecond,
					input: commandData{
						startTime: 100 * time.Millisecond,
						endTime:   300 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("exit"), timestamp: 200 * time.Millisecond},
						},
					},
					output: commandData{
						startTime: 300 * time.Millisecond,
						endTime:   400 * time.Millisecond,
						startSize: size{cols: 80, rows: 24},
						tokens: []token{
							{tokenType: tokenText, data: []byte("goodbye\n"), timestamp: 400 * time.Millisecond},
						},
					},
				},
			},
			description: "command emitted when parser closes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			parser := newParser()

			tokenChan := createTokenStream(ctx, tt.tokens)

			processDone := make(chan error, 1)
			go func() {
				processDone <- parser.processTokens(ctx, tokenChan)
			}()

			commands := collectCommands(t, parser, processDone, 2*time.Second)

			require.Len(t, commands, len(tt.expectedCommands),
				"Scenario: %s - Expected %d commands but got %d",
				tt.description, len(tt.expectedCommands), len(commands))

			for i, expected := range tt.expectedCommands {
				if i >= len(commands) {
					break
				}

				actual := commands[i]
				require.Equal(t, expected, actual,
					"command %d mismatch in scenario: %s", i, tt.description)
			}
		})
	}
}

func TestParserErrorHandling(t *testing.T) {
	t.Run("unexpected_bracket_paste_end", func(t *testing.T) {
		ctx := t.Context()
		parser := newParser()

		tokens := []token{
			{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
			{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 100 * time.Millisecond},
		}

		tokenChan := createTokenStream(ctx, tokens)

		err := parser.processTokens(ctx, tokenChan)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected bracket paste end without a current command")
	})

	t.Run("mismatched_bracket_paste", func(t *testing.T) {
		ctx := t.Context()
		parser := newParser()

		tokens := []token{
			{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
			{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
			{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 200 * time.Millisecond},
			{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 300 * time.Millisecond},
		}

		tokenChan := createTokenStream(ctx, tokens)

		err := parser.processTokens(ctx, tokenChan)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected bracket paste end without matching start")
	})

	t.Run("invalid_resize_format", func(t *testing.T) {
		ctx := t.Context()
		parser := newParser()

		tokens := []token{
			{tokenType: tokenResize, data: []byte("invalid"), timestamp: 0},
		}

		tokenChan := createTokenStream(ctx, tokens)

		err := parser.processTokens(ctx, tokenChan)
		require.Error(t, err)
	})

	t.Run("context_cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		parser := newParser()

		tokenChan := make(chan token)

		processDone := make(chan error)
		go func() {
			processDone <- parser.processTokens(ctx, tokenChan)
		}()

		cancel()

		select {
		case err := <-processDone:
			require.Error(t, err)
			require.Contains(t, err.Error(), "context canceled")
		case <-time.After(1 * time.Second):
			t.Fatal("Test timed out waiting for context cancellation")
		}
	})

	t.Run("text_without_command", func(t *testing.T) {
		ctx := t.Context()
		parser := newParser()

		tokens := []token{
			{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
			{tokenType: tokenText, data: []byte("orphaned text"), timestamp: 100 * time.Millisecond},
		}

		tokenChan := createTokenStream(ctx, tokens)

		err := parser.processTokens(ctx, tokenChan)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no current command to append token to")
	})
}

func TestParserAlternateScreenInteractions(t *testing.T) {
	t.Run("alternate_screen_during_output", func(t *testing.T) {
		ctx := t.Context()
		parser := newParser()

		tokens := []token{
			{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
			{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
			{tokenType: tokenText, data: []byte("vim"), timestamp: 200 * time.Millisecond},
			{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 300 * time.Millisecond},
			{tokenType: tokenAlternateScreenEnter, data: []byte("\x1b[?1049h"), timestamp: 250 * time.Millisecond},
			{tokenType: tokenText, data: []byte("editing"), timestamp: 400 * time.Millisecond},
			{tokenType: tokenAlternateScreenExit, data: []byte("\x1b[?1049l"), timestamp: 500 * time.Millisecond},
		}

		tokenChan := createTokenStream(ctx, tokens)

		processDone := make(chan error, 1)
		go func() {
			processDone <- parser.processTokens(ctx, tokenChan)
		}()

		commands := collectCommands(t, parser, processDone, 2*time.Second)

		require.Len(t, commands, 1)
		cmd := commands[0]

		require.False(t, cmd.input.isAlternateScreen, "input should not be marked as alternate screen")
		require.True(t, cmd.output.isAlternateScreen, "Output should be marked as alternate screen")
	})

	t.Run("multiple_alternate_screen_transitions", func(t *testing.T) {
		ctx := t.Context()
		parser := newParser()

		tokens := []token{
			{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
			{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
			{tokenType: tokenText, data: []byte("command"), timestamp: 200 * time.Millisecond},
			{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 300 * time.Millisecond},
			{tokenType: tokenAlternateScreenEnter, data: []byte("\x1b[?1049h"), timestamp: 400 * time.Millisecond},
			{tokenType: tokenAlternateScreenExit, data: []byte("\x1b[?1049l"), timestamp: 500 * time.Millisecond},
			{tokenType: tokenAlternateScreenEnter, data: []byte("\x1b[?1049h"), timestamp: 600 * time.Millisecond},
			{tokenType: tokenText, data: []byte("in alternate"), timestamp: 700 * time.Millisecond},
			{tokenType: tokenAlternateScreenExit, data: []byte("\x1b[?1049l"), timestamp: 800 * time.Millisecond},
		}

		tokenChan := createTokenStream(ctx, tokens)

		processDone := make(chan error, 1)
		go func() {
			processDone <- parser.processTokens(ctx, tokenChan)
		}()

		commands := collectCommands(t, parser, processDone, 2*time.Second)

		require.Len(t, commands, 1)
		cmd := commands[0]

		require.True(t, cmd.output.isAlternateScreen, "Output should be marked as alternate screen")
		require.Contains(t, string(cmd.output.tokens[0].data), "in alternate")
	})
}

func TestParserEdgeCases(t *testing.T) {
	t.Run("completely_empty_command", func(t *testing.T) {
		ctx := t.Context()
		parser := newParser()

		tokens := []token{
			{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
			{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
			{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 200 * time.Millisecond},
			// Next command with content
			{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 300 * time.Millisecond},
			{tokenType: tokenText, data: []byte("ls"), timestamp: 400 * time.Millisecond},
			{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 500 * time.Millisecond},
		}

		tokenChan := createTokenStream(ctx, tokens)

		processDone := make(chan error, 1)
		go func() {
			processDone <- parser.processTokens(ctx, tokenChan)
		}()

		commands := collectCommands(t, parser, processDone, 2*time.Second)

		// Empty commands should not be emitted
		require.Len(t, commands, 1, "Should only emit non-empty command")
		require.Equal(t, "ls", string(commands[0].input.tokens[0].data))
	})

	t.Run("resize_without_active_command", func(t *testing.T) {
		ctx := t.Context()
		parser := newParser()

		tokens := []token{
			{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
			{tokenType: tokenResize, data: []byte("120:40"), timestamp: 100 * time.Millisecond},
			{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 200 * time.Millisecond},
			{tokenType: tokenText, data: []byte("command"), timestamp: 300 * time.Millisecond},
			{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 400 * time.Millisecond},
		}

		tokenChan := createTokenStream(ctx, tokens)

		processDone := make(chan error, 1)
		go func() {
			processDone <- parser.processTokens(ctx, tokenChan)
		}()

		commands := collectCommands(t, parser, processDone, 2*time.Second)

		require.Len(t, commands, 1)
		// Should use the size from when command started
		require.Equal(t, 120, commands[0].input.startSize.cols)
		require.Equal(t, 40, commands[0].input.startSize.rows)
	})

	t.Run("deeply_nested_bracket_paste", func(t *testing.T) {
		ctx := t.Context()
		parser := newParser()

		tokens := []token{
			{tokenType: tokenResize, data: []byte("80:24"), timestamp: 0},
			{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 100 * time.Millisecond},
			{tokenType: tokenText, data: []byte("level1"), timestamp: 150 * time.Millisecond},
			{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 200 * time.Millisecond},
			{tokenType: tokenText, data: []byte("level2"), timestamp: 250 * time.Millisecond},
			{tokenType: tokenBracketPasteStart, data: []byte("\x1b[?2004h"), timestamp: 300 * time.Millisecond},
			{tokenType: tokenText, data: []byte("level3"), timestamp: 350 * time.Millisecond},
			{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 400 * time.Millisecond},
			{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 450 * time.Millisecond},
			{tokenType: tokenBracketPasteEnd, data: []byte("\x1b[?2004l"), timestamp: 500 * time.Millisecond},
			{tokenType: tokenText, data: []byte("output"), timestamp: 600 * time.Millisecond},
		}

		tokenChan := createTokenStream(ctx, tokens)

		processDone := make(chan error, 1)
		go func() {
			processDone <- parser.processTokens(ctx, tokenChan)
		}()

		commands := collectCommands(t, parser, processDone, 2*time.Second)

		require.Len(t, commands, 1)
		cmd := commands[0]

		// Should have all nested content in input
		require.Len(t, cmd.input.tokens, 7) // 3 text + 2 paste starts + 2 paste ends
		require.Equal(t, "output", string(cmd.output.tokens[0].data))
	})
}

func createTokenStream(ctx context.Context, tokens []token) chan token {
	tokenChan := make(chan token)

	go func() {
		defer close(tokenChan)

		for _, tok := range tokens {
			select {
			case <-ctx.Done():
				return
			case tokenChan <- tok:
			}
		}
	}()

	return tokenChan
}

func collectCommands(t *testing.T, parser *parser, processDone <-chan error, timeout time.Duration) []*command {
	timeoutChan := time.After(timeout)

	select {
	case err := <-processDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("Processing error: %v", err)
		}
	case <-timeoutChan:
		t.Fatal("Test timed out waiting for processing")
	}

	var commands []*command
	for cmd := range parser.commands() {
		commands = append(commands, cmd)
	}

	return commands
}
