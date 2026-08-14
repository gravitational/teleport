package tokenizer

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestProperty_Tokens_NeverPanicsOnArbitraryInput(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the real BPE file (network download when TELEPORT_TIKTOKEN_BPE_DIR is unset)")
	}

	if _, err := initTokenizer(); err != nil {
		t.Skipf("could not initialize the real encoder, skipping: %v", err)
	}

	rapid.Check(t, func(t *rapid.T) {
		text := rapid.OneOf(
			rapid.String(),
			rapid.Map(rapid.SliceOf(rapid.Byte()), func(b []byte) string { return string(b) }),
		).Draw(t, "text")

		tokens, err := EncodeTokens(text)
		require.NoError(t, err)

		require.Equal(t, len(tokens), Counter{}.CountTokens(text))

		_, err = DecodeTokens(tokens)
		require.NoError(t, err)
	})
}
