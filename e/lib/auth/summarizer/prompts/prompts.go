package prompts

import (
	_ "embed"
	rand "math/rand/v2"
)

var (
	// The raw prompt variables are obfuscated so that they can't be easily
	// extracted as plain strings from Teleport executable. We do it to make it
	// harder for people to exploit prompt weaknesses.

	//go:embed prompt-ssh.bin
	sshPromptObfuscated []byte
	//go:embed prompt-db.bin
	databasePromptObfuscated []byte

	// Prompt for summarizing SSH sessions.
	SSHPrompt string
	// Prompt for summarizing database sessions.
	DatabasePrompt string
)

func init() {
	SSHPrompt = deobfuscate(sshPromptObfuscated)
	DatabasePrompt = deobfuscate(databasePromptObfuscated)
}

// Obfuscates a byte slice by XOR'ing it with a random sequence of a given
// seed.
func Obfuscate(seed uint64, p []byte) []byte {
	return transform(seed, p)
}

func deobfuscate(p []byte) string {
	return string(transform(promptSeed, p))
}

func transform(seed uint64, in []byte) []byte {
	rng := rand.New(rand.NewPCG(seed, seed))
	out := make([]byte, len(in))
	for i, b := range in {
		out[i] = b ^ byte(rng.UintN(256))
	}
	return out
}
