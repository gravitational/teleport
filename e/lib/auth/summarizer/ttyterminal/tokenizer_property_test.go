package ttyterminal

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

func TestProperty_Tokenize_NeverPanicsOnRandomData(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		data := genAdversarialBytes(t, "data", 4096)

		testutils.RunWithTimeout(t, 500*time.Millisecond, func() {
			tk, done := drainedTokenizer()
			defer done()

			ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
			defer cancel()

			_ = tk.processPrintEvent(ctx, &apievents.SessionPrint{
				DelayMilliseconds: 0,
				Data:              data,
			})
		})
	})
}

func TestProperty_Tokenize_NeverPanicsOnRandomEventSequence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(0, 10).Draw(t, "count")
		events := make([]*apievents.SessionPrint, count)
		for i := range events {
			events[i] = &apievents.SessionPrint{
				DelayMilliseconds: int64(rapid.IntRange(0, 60_000).Draw(t, "delay")),
				Data:              genAdversarialBytes(t, "data", 256),
			}
		}

		testutils.RunWithTimeout(t, 1*time.Second, func() {
			tk, done := drainedTokenizer()
			defer done()

			ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
			defer cancel()

			for _, evt := range events {
				if err := tk.processPrintEvent(ctx, evt); err != nil {
					return
				}
			}
		})
	})
}

func TestProperty_Tokenize_NeverPanicsOnResizeEvents(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		size := rapid.OneOf(
			rapid.Just(""),
			rapid.Just("0:0"),
			rapid.Just("80:24"),
			rapid.Just("99999999:99999999"),
			rapid.StringN(0, 64, -1),
		).Draw(t, "size")

		testutils.RunWithTimeout(t, 500*time.Millisecond, func() {
			tk, done := drainedTokenizer()
			defer done()

			ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
			defer cancel()

			_ = tk.processResizeEvent(ctx, &apievents.Resize{
				TerminalSize: size,
			})
		})
	})
}

func TestProperty_Tokenize_TokensReconstructInput(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		var input []byte
		segments := rapid.IntRange(1, 8).Draw(t, "segments")
		for i := 0; i < segments; i++ {
			input = append(input, rapid.SampledFrom(specialSequences).Draw(t, "seq").pattern...)
			input = append(input, genTextNoEscape(t, "text")...)
		}

		tk := newTokenizer()

		var got []byte
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for tok := range tk.tokens() {
				got = append(got, tok.data...)
			}
		}()

		require.NoError(t, tk.processPrintEvent(t.Context(), &apievents.SessionPrint{Data: input}))
		tk.close()
		wg.Wait()

		require.Equal(t, input, got)
	})
}

func genTextNoEscape(t *rapid.T, label string) []byte {
	t.Helper()

	return rapid.SliceOfN(
		rapid.Byte().Filter(func(b byte) bool { return b != escapeChar }),
		0, 32,
	).Draw(t, label)
}

// genAdversarialBytes generates byte slices biased toward ANSI escape introducers, common control bytes, and boundary values.
func genAdversarialBytes(t *rapid.T, label string, maxLen int) []byte {
	t.Helper()

	return rapid.SliceOfN(
		rapid.OneOf(
			rapid.Just(byte(0x1b)), // ESC
			rapid.Just(byte(0x9b)), // CSI (8-bit)
			rapid.Just(byte(0x00)), // NUL
			rapid.Just(byte(0x07)), // BEL (string terminator in some sequences)
			rapid.Just(byte('[')),  // CSI introducer body
			rapid.Just(byte('?')),  // private-mode prefix
			rapid.Just(byte('h')),  // set-mode final
			rapid.Just(byte('l')),  // reset-mode final
			rapid.Just(byte(']')),  // OSC introducer
			rapid.Just(byte(0xff)),
			rapid.Byte(), // uniform fallback
		),
		0, maxLen,
	).Draw(t, label)
}

// drainedTokenizer creates a tokenizer with a background goroutine draining the token channel until close.
func drainedTokenizer() (*tokenizer, func()) {
	tk := newTokenizer()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for range tk.tokens() {
		}
	}()

	return tk, func() {
		tk.close()
		wg.Wait()
	}
}
