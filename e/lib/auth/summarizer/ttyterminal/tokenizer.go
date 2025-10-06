package ttyterminal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
)

type tokenType int

const (
	tokenText tokenType = iota
	tokenBracketPasteStart
	tokenBracketPasteEnd
	tokenAlternateScreenEnter
	tokenAlternateScreenExit
	tokenResize
)

// tokenizer processes a stream of SSH/k8s session recording events and splits each event's data
// into separate tokens. When control sequences (like alternate screen or bracketed paste
// escape codes) are found within an event, the tokenizer emits multiple tokens: one for
// each text segment and control sequence encountered. This one-to-many transformation
// simplifies downstream processing by isolating special terminal behaviors from regular output.
// This can be further extended to handle heuristic command detection and segmentation, for older
// shells that do not emit bracketed paste sequences.
type tokenizer struct {
	tokenChan chan token
	closeOnce sync.Once
	startTime time.Time
}

type token struct {
	tokenType tokenType
	data      []byte
	timestamp time.Duration
}

const tokenBufferSize = 1024

func newTokenizer() *tokenizer {
	return &tokenizer{
		tokenChan: make(chan token, tokenBufferSize),
	}
}

func (t *tokenizer) processEventStream(ctx context.Context, events <-chan apievents.AuditEvent, errs <-chan error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				t.close()
				return nil
			}

			switch e := evt.(type) {
			case *apievents.SessionStart:
				if err := t.processStartEvent(ctx, e); err != nil {
					return err
				}

			case *apievents.SessionPrint:
				if err := t.processPrintEvent(ctx, e); err != nil {
					return err
				}

			case *apievents.Resize:
				if err := t.processResizeEvent(ctx, e); err != nil {
					return err
				}

			case *apievents.SessionEnd:
				t.close()
				return nil
			}

		case err := <-errs:
			t.close()

			if err != nil && !errors.Is(err, io.EOF) {
				return trace.Wrap(err)
			}

			return nil

		case <-ctx.Done():
			t.close()
			return ctx.Err()
		}
	}
}

func (t *tokenizer) tokens() <-chan token {
	return t.tokenChan
}

func (t *tokenizer) processStartEvent(ctx context.Context, e *apievents.SessionStart) error {
	t.startTime = e.Time

	return trace.Wrap(t.emitToken(ctx, tokenResize, []byte(e.TerminalSize), 0))
}

func (t *tokenizer) processPrintEvent(ctx context.Context, e *apievents.SessionPrint) error {
	timestamp := time.Duration(e.DelayMilliseconds) * time.Millisecond
	data := e.Data

	return trace.Wrap(t.tokenizeData(ctx, data, timestamp))
}

func (t *tokenizer) processResizeEvent(ctx context.Context, e *apievents.Resize) error {
	timestamp := e.Time.Sub(t.startTime)

	return trace.Wrap(t.emitToken(ctx, tokenResize, []byte(e.TerminalSize), timestamp))
}

type sequencePattern struct {
	pattern []byte
	token   tokenType
}

// specialSequences defines the control sequences that the tokenizer recognizes and emits as separate tokens.
// This isn't the most efficient way to do this, but it's simple and effective for the limited set of sequences we care about.
// This can be replaced with a trie implementation if performance becomes a concern.
// Parsing the sequences inline does not perform much better than looping through the list, and is more unwieldy.
var specialSequences = []sequencePattern{
	{[]byte("\x1b[?2004h"), tokenBracketPasteStart},
	{[]byte("\x1b[?2004l"), tokenBracketPasteEnd},
	{[]byte("\x1b[?1049h"), tokenAlternateScreenEnter},
	{[]byte("\x1b[?1047h"), tokenAlternateScreenEnter},
	{[]byte("\x1b[?47h"), tokenAlternateScreenEnter},
	{[]byte("\x1b[?1049l"), tokenAlternateScreenExit},
	{[]byte("\x1b[?1047l"), tokenAlternateScreenExit},
	{[]byte("\x1b[?47l"), tokenAlternateScreenExit},
}

func (t *tokenizer) tokenizeData(ctx context.Context, data []byte, timestamp time.Duration) error {
	pos := 0
	searchPos := 0

	for searchPos < len(data) {
		escapePos := bytes.IndexByte(data[searchPos:], '\x1b')
		if escapePos == -1 {
			break
		}

		escapePos += searchPos
		matched := false

		for _, seq := range specialSequences {
			if !bytes.HasPrefix(data[escapePos:], seq.pattern) {
				continue
			}

			if escapePos > pos {
				if err := t.emitToken(ctx, tokenText, data[pos:escapePos], timestamp); err != nil {
					return trace.Wrap(err)
				}
			}

			if err := t.emitToken(ctx, seq.token, seq.pattern, timestamp); err != nil {
				return trace.Wrap(err)
			}

			pos = escapePos + len(seq.pattern)
			searchPos = pos
			matched = true

			break
		}

		if !matched {
			searchPos = escapePos + 1
		}
	}

	if pos < len(data) {
		if err := t.emitToken(ctx, tokenText, data[pos:], timestamp); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (t *tokenizer) emitToken(ctx context.Context, tokenType tokenType, data []byte, timestamp time.Duration) error {
	tok := token{
		tokenType: tokenType,
		data:      data,
		timestamp: timestamp,
	}

	select {
	case t.tokenChan <- tok:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *tokenizer) close() {
	t.closeOnce.Do(func() {
		close(t.tokenChan)
	})
}
