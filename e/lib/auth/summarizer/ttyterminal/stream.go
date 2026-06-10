package ttyterminal

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"

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

	g.Go(recoverAsError(gCtx, func() error {
		return tokenizer.processEventStream(gCtx, evts, streamErrs)
	}))

	// Peek into the token stream to see if bracketed paste mode escape sequences are
	// present. If not, we skip the parsing and command recreation stages entirely
	peeked, replayedTokens, err := peekTokens(gCtx, tokens, peekTokenCount)
	if err != nil {
		if realErr := drainPeekPipeline(cancel, g); realErr != nil {
			return nil, trace.Wrap(realErr)
		}
		return nil, trace.Wrap(err)
	}

	if !detectBracketedPaste(peeked) {
		if realErr := drainPeekPipeline(cancel, g); realErr != nil {
			return nil, trace.Wrap(realErr)
		}
		return &TTYRecordingStream{}, nil
	}

	recreatedChan := make(chan Command, commandBufferSize)
	parser := newParser()

	g.Go(recoverAsError(gCtx, func() error {
		return parser.processTokens(gCtx, replayedTokens)
	}))

	g.Go(recoverAsError(gCtx, func() error {
		defer close(recreatedChan)
		return processCommands(gCtx, parser.commands(), recreatedChan)
	}))

	return &TTYRecordingStream{
		commands: recreatedChan,
		g:        g,
		cancel:   cancel,
	}, nil
}

// drainPeekPipeline tears down the pipeline and returns the worker's real error,
// if any. Context cancellation/deadline is ignored since it's just the result of
// our own cancel(); a real stream error lives in the errgroup, not in peekTokens.
func drainPeekPipeline(cancel context.CancelFunc, g *errgroup.Group) error {
	cancel()
	if err := g.Wait(); err != nil &&
		!errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return nil
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

// detectBracketedPaste reports whether the peeked tokens look like a session where
// the shell uses bracketed paste mode to delimit commands. A bracketed paste token
// must appear before any alternate screen enter — that filters out TUI apps
// (vim, less, htop) which emit \x1b[?2004h for their own paste handling.
func detectBracketedPaste(peeked []token) bool {
	for _, token := range peeked {
		switch token.tokenType {
		case tokenAlternateScreenEnter:
			return false
		case tokenBracketPasteStart, tokenBracketPasteEnd:
			return true
		}
	}
	return false
}

// errInternalProcessing is the error [recoverAsError] returns when a pipeline worker panics. It is a
// package-level sentinel so callers can match it with [errors.Is]; its message is shown to end users.
var errInternalProcessing = errors.New("internal error while processing session recording")

// recoverAsError wraps a pipeline worker as a last-resort safety net so any unexpected panic becomes a normal error
// that the errgroup surfaces, rather than crashing the auth server. Panics from vt10x are normally caught per-command
// inside [reconstructCommand] — that preserves partial summarization of the surrounding commands — so reaching this
// handler means an unexpected panic elsewhere in the pipeline.
//
// The stack trace is logged server-side only. The returned error is intentionally stack-free because it flows
// into summarizer.handleError, which stores err.Error() on Summary.ErrorMessage — a field exposed to end users.
func recoverAsError(ctx context.Context, fn func() error) func() error {
	return func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(ctx, "panic in ttyterminal stream",
					"panic", r,
					"stack", string(debug.Stack()),
				)
				err = trace.Wrap(errInternalProcessing)
			}
		}()

		return trace.Wrap(fn())
	}
}
