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

const escapeChar = '\x1b'

// tokenizer processes a stream of SSH/k8s session recording events and splits each event's data
// into separate tokens. When control sequences (like alternate screen or bracketed paste
// escape codes) are found within an event, the tokenizer emits multiple tokens: one for
// each text segment and control sequence encountered. This one-to-many transformation
// simplifies downstream processing by isolating special terminal behaviors from regular output.
// This can be further extended to handle heuristic command detection and segmentation, for older
// shells that do not emit bracketed paste sequences.
// Note: any text before the first escape character is ignored, to skip over MOTD and other non-interactive output,
// which means the full session could be read without emitting any tokens if no escape characters are found.
type tokenizer struct {
	tokenChan       chan token
	closeOnce       sync.Once
	startTime       time.Time
	hasEscape       bool
	inTitleSequence bool
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

	if !t.hasEscape {
		escapeIdx := bytes.IndexByte(data, escapeChar)
		if escapeIdx == -1 {
			// Ignore print events until we see an escape character, avoiding
			// processing the MOTD or other non-interactive output.
			return nil
		}

		// Skip any data before the first escape character
		t.hasEscape = true
		data = data[escapeIdx:]
	}

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

	if t.inTitleSequence {
		if end := findTitleSequenceTerminator(data); end > 0 {
			pos = end
			searchPos = end
			t.inTitleSequence = false
		} else {
			// We're still in a title sequence, skip all data
			return nil
		}
	}

	for searchPos < len(data) {
		escapePos := bytes.IndexByte(data[searchPos:], escapeChar)
		if escapePos == -1 {
			break
		}

		escapePos += searchPos
		matched := false

		if titleEnd, complete := findTitleSequenceEnd(data[escapePos:]); titleEnd > 0 {
			// If we found a title sequence, emit any preceding text
			if escapePos > pos {
				if err := t.emitToken(ctx, tokenText, data[pos:escapePos], timestamp); err != nil {
					return trace.Wrap(err)
				}
			}

			// Skip the entire title sequence
			pos = escapePos + titleEnd
			searchPos = pos

			if !complete {
				// Title sequence continues in next event
				t.inTitleSequence = true
				return nil
			}

			continue
		}

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

// findTitleSequenceEnd checks if data starts with an OSC sequence (\x1b]<digits>...)
// and returns the length of the sequence to skip and whether the sequence is complete.
// All OSC sequences are stripped, not just title sequences — shell integration
// markers such as OSC 133, OSC 633 and OSC 3008 also appear at session start.
func findTitleSequenceEnd(data []byte) (int, bool) {
	if len(data) < 3 {
		return 0, false
	}

	if data[0] != escapeChar || data[1] != ']' {
		return 0, false
	}

	// OSC parameter is one or more digits.
	if data[2] < '0' || data[2] > '9' {
		return 0, false
	}

	if end := findTitleSequenceTerminator(data[3:]); end > 0 {
		return end + 3, true
	}

	return len(data), false
}

// findTitleSequenceTerminator looks for the terminator of an ongoing title sequence.
// Returns the position after the terminator, or 0 if no terminator is found.
func findTitleSequenceTerminator(data []byte) int {
	for i := 0; i < len(data); i++ {
		if data[i] == '\x07' {
			return i + 1
		}

		if data[i] == escapeChar && i+1 < len(data) && data[i+1] == '\\' {
			return i + 2
		}
	}

	return 0
}
