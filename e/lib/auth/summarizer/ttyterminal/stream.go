package ttyterminal

import (
	"context"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	apievents "github.com/gravitational/teleport/api/types/events"
)

const (
	commandBufferSize = 32
	peekTokenCount    = 10

	inputMaxTokensPerChunk = 10000
	inputChunkLimit        = 5

	outputMaxTokensPerChunk = 5000
	outputChunkLimit        = 10
)

// StreamTTYRecording processes a stream of the events from a session recording through a three-stage pipeline:
// tokenizer -> parser -> recreator. It returns stream of recreated commands.
func StreamTTYRecording(ctx context.Context, evts <-chan apievents.AuditEvent, streamErrs <-chan error) (*TTYRecordingStream, error) {
	ctx, cancel := context.WithCancel(ctx)

	tokenizer := newTokenizer()
	tokens := tokenizer.tokens()

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return tokenizer.processEventStream(gCtx, evts, streamErrs)
	})

	// Peek into the token stream to see if bracketed paste mode escape sequences are
	// present. If not, we skip the parsing and command recreation stages entirely
	peeked, replayedTokens, err := peekTokens(gCtx, tokens, peekTokenCount)
	if err != nil {
		cancel()
		g.Wait()
		return nil, trace.Wrap(err)
	}

	if !detectBracketedPaste(peeked) {
		cancel()
		g.Wait()
		return &TTYRecordingStream{}, nil
	}

	recreatedChan := make(chan Command, commandBufferSize)
	parser := newParser()

	g.Go(func() error {
		return parser.processTokens(gCtx, replayedTokens)
	})

	g.Go(func() error {
		defer close(recreatedChan)
		return processCommands(gCtx, parser.commands(), recreatedChan)
	})

	return &TTYRecordingStream{
		commands: recreatedChan,
		g:        g,
		cancel:   cancel,
	}, nil
}

// TTYRecordingStream represents a processing stream for session recording data.
// If bracketed paste mode sequences are not detected, the commands channel will be nil.
type TTYRecordingStream struct {
	commands <-chan Command
	g        *errgroup.Group
	cancel   context.CancelFunc
}

// Close cancels the processing of the stream and waits for completion.
func (s *TTYRecordingStream) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return s.Wait()
}

// Commands returns a channel of reconstructed commands from the session recording.
func (s *TTYRecordingStream) Commands() <-chan Command {
	return s.commands
}

// HasCommands indicates whether the stream contains reconstructed commands.
func (s *TTYRecordingStream) HasCommands() bool {
	return s.commands != nil
}

// Wait waits for the processing goroutines to complete and returns any error encountered.
func (s *TTYRecordingStream) Wait() error {
	if s.g == nil {
		return nil
	}
	return s.g.Wait()
}

func processCommands(ctx context.Context, commandChan <-chan *command, recreatedChan chan<- Command) error {
	for {
		select {
		case cmd, ok := <-commandChan:
			if !ok {
				return nil
			}

			select {
			case recreatedChan <- processCommand(cmd):
			case <-ctx.Done():
				return ctx.Err()
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func processCommand(cmd *command) *ReconstructedCommand {
	inputData := reconstructCommand(cmd.input, inputMaxTokensPerChunk, inputChunkLimit)
	outputData := reconstructCommand(cmd.output, outputMaxTokensPerChunk, outputChunkLimit)

	return &ReconstructedCommand{
		input:  inputData,
		output: outputData,
	}
}

// peekTokens reads up to 'max' tokens from the input channel without consuming them,
// returning the peeked tokens and a new channel that replays them followed by the remaining tokens.
func peekTokens(ctx context.Context, tokens <-chan token, max int) ([]token, <-chan token, error) {
	peeked := make([]token, 0, max)

	for range max {
		select {
		case token, ok := <-tokens:
			if !ok {
				// If we reach the end of the channel, we consult the context for cancellation
				// since the other goroutine may have exited and closed the channel on cancellation.
				if err := ctx.Err(); err != nil {
					return nil, nil, trace.Wrap(err)
				}
				return peeked, replayTokens(ctx, peeked, tokens), nil
			}
			peeked = append(peeked, token)
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}

	return peeked, replayTokens(ctx, peeked, tokens), nil
}

// replayTokens creates a new channel that first emits the peeked tokens
// followed by the remaining tokens from the original channel.
func replayTokens(ctx context.Context, peeked []token, remaining <-chan token) <-chan token {
	replayed := make(chan token, len(peeked))

	go func() {
		defer close(replayed)

		for _, token := range peeked {
			select {
			case replayed <- token:
			case <-ctx.Done():
				return
			}
		}

		for {
			select {
			case token, ok := <-remaining:
				if !ok {
					return
				}
				select {
				case replayed <- token:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return replayed
}

// detectBracketedPaste checks if the first non-resize token is a bracketed paste mode sequence.
func detectBracketedPaste(peeked []token) bool {
	for _, token := range peeked {
		if token.tokenType == tokenResize {
			continue
		}
		return token.tokenType == tokenBracketPasteStart || token.tokenType == tokenBracketPasteEnd
	}
	return false
}
