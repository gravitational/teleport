package ttyterminal

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/lib/utils/testutils"
)

func TestProperty_Parser_NeverPanicsOnRandomTokenSequence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(0, 50).Draw(t, "count")
		tokens := make([]token, count)
		for i := range tokens {
			tokens[i] = genToken(t, "tok")
		}

		testutils.RunWithTimeout(t, 500*time.Millisecond, func() {
			p, done := drainedParser()
			defer done()
			for _, tok := range tokens {
				if err := p.processToken(tok); err != nil {
					return
				}
			}
		})
	})
}

func TestProperty_Parser_BracketPasteDepthNeverNegative(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Specifically generate sequences biased toward bracket-paste imbalance: extra ends, repeated starts, etc.
		count := rapid.IntRange(0, 50).Draw(t, "count")
		tokens := make([]token, count)
		for i := range tokens {
			tt := rapid.SampledFrom([]tokenType{
				tokenBracketPasteStart,
				tokenBracketPasteEnd,
				tokenBracketPasteEnd, // weight ends higher to provoke imbalance
				tokenText,
			}).Draw(t, "tt")
			tokens[i] = token{tokenType: tt, data: nil, timestamp: 0}
		}

		p, done := drainedParser()
		defer done()

		for _, tok := range tokens {
			_ = p.processToken(tok)
			if p.bracketPasteDepth < 0 {
				t.Fatalf("bracketPasteDepth went negative after %d tokens", len(tokens))
			}
		}
	})
}

func TestProperty_Parser_BalancedBracketPasteReturnsToZero(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		depth := rapid.IntRange(1, 20).Draw(t, "depth")
		p, done := drainedParser()
		defer done()

		for i := 0; i < depth; i++ {
			require.NoError(t, p.processToken(token{tokenType: tokenBracketPasteStart}))
		}
		for i := 0; i < depth; i++ {
			require.NoError(t, p.processToken(token{tokenType: tokenBracketPasteEnd}))
		}

		require.Zero(t, p.bracketPasteDepth)
		require.Equal(t, stateCapturingOutput, p.state)
	})
}

// genToken produces a random token. Bias toward control sequences so the bracket-paste / alternate-screen state machine paths get exercised.
func genToken(t *rapid.T, label string) token {
	t.Helper()

	tt := rapid.SampledFrom([]tokenType{
		tokenText,
		tokenBracketPasteStart,
		tokenBracketPasteEnd,
		tokenAlternateScreenEnter,
		tokenAlternateScreenExit,
		tokenResize,
	}).Draw(t, label+"_type")

	var data []byte
	switch tt {
	case tokenResize:
		data = []byte(rapid.OneOf(
			rapid.Just(""),
			rapid.Just("80:24"),
			rapid.Just("0:0"),
			rapid.Just("not-a-size"),
			rapid.StringN(0, 32, -1),
		).Draw(t, label+"_resize_data"))

	default:
		data = rapid.SliceOfN(rapid.Byte(), 0, 128).Draw(t, label+"_data")
	}

	timestampMs := rapid.Int64Range(0, int64(time.Hour/time.Millisecond)).Draw(t, label+"_ts")
	return token{
		tokenType: tt,
		data:      data,
		timestamp: time.Duration(timestampMs) * time.Millisecond,
	}
}

// drainedParser builds a parser with a background goroutine draining the commands channel until close.
func drainedParser() (*parser, func()) {
	p := newParser()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for range p.commands() {
		}
	}()

	return p, func() {
		p.cleanup()
		wg.Wait()
	}
}
