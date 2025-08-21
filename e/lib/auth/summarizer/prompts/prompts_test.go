package prompts

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptsUpToDate(t *testing.T) {
	cases := []struct {
		filename         string
		prompt           string
		obfuscatedPrompt []byte
	}{
		{
			filename:         "prompt-db.txt",
			prompt:           DatabasePrompt,
			obfuscatedPrompt: databasePromptObfuscated,
		},
		{
			filename:         "prompt-ssh.txt",
			prompt:           SSHPrompt,
			obfuscatedPrompt: sshPromptObfuscated,
		},
	}
	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			assert.NotEqual(t, tc.prompt, string(tc.obfuscatedPrompt))
			text, err := os.ReadFile(tc.filename)
			require.NoError(t, err)
			assert.Equal(t, tc.prompt, string(text),
				"The obfuscated prompts are not up-to-date with their sources. Please run:\n"+
					"make -C e obfuscate-prompts",
			)
		})
	}
}
