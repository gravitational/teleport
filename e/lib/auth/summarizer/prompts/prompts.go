package prompts

import (
	_ "embed"
	"math/rand/v2"
)

var (
	// The raw prompt variables are obfuscated so that they can't be easily
	// extracted as plain strings from Teleport executable. We do it to make it
	// harder for people to exploit prompt weaknesses.

	//go:embed prompt-ssh.bin
	sshPromptObfuscated []byte
	//go:embed prompt-db.bin
	databasePromptObfuscated []byte
	//go:embed prompt-command.bin
	commandPromptObfuscated []byte
	//go:embed prompt-root.bin
	rootPromptObfuscated []byte
	//go:embed prompt-summary.bin
	summaryPromptObfuscated []byte
	//go:embed prompt-proser.bin
	proserPromptObfuscated []byte
	//go:embed prompt-screenshots.bin
	screenshotsPromptObfuscated []byte
	//go:embed prompt-screenshots-synthesis.bin
	screenshotsSynthesisPromptObfuscated []byte

	// Prompt for summarizing SSH sessions.
	SSHPrompt string
	// Prompt for summarizing database sessions.
	DatabasePrompt string
	// Prompt for summarizing commands.
	CommandPrompt string
	// Prompt to add to the command prompt when the user is root.
	RootPrompt string
	// Prompt for summarizing multiple commands in a session.
	SummaryPrompt string
	// Prompt for generating a dense embedding of a session.
	ProserPrompt string
	// Prompt fragment for summarizing batches of desktop screenshots.
	ScreenshotsPrompt string
	// Prompt for synthesizing per-screenshot events into a desktop session-level analysis.
	ScreenshotsSynthesisPrompt string
)

func init() {
	SSHPrompt = deobfuscate(sshPromptObfuscated)
	DatabasePrompt = deobfuscate(databasePromptObfuscated)
	CommandPrompt = deobfuscate(commandPromptObfuscated)
	RootPrompt = deobfuscate(rootPromptObfuscated)
	SummaryPrompt = deobfuscate(summaryPromptObfuscated)
	ProserPrompt = deobfuscate(proserPromptObfuscated)
	ScreenshotsPrompt = deobfuscate(screenshotsPromptObfuscated)
	ScreenshotsSynthesisPrompt = deobfuscate(screenshotsSynthesisPromptObfuscated)
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
