package schema

import (
	"fmt"

	"github.com/gravitational/teleport/e/lib/auth/summarizer/prompts"
)

// SummarizeCommandSystemPrompt generates the system prompt for command summarization
// based on the user's username and login name.
func SummarizeCommandSystemPrompt(username, loginName string) string {
	rootSuffix := ""
	if loginName == "root" {
		rootSuffix = prompts.RootPrompt
	}

	return fmt.Sprintf(
		"You are analyzing a single command from a session recording for user %q (logged in as %q).\n\n%s%s",
		username,
		loginName,
		prompts.CommandPrompt,
		rootSuffix,
	)
}

// SummarizeMultipleCommandsSystemPrompt generates the system prompt for summarization of
// multiple command summaries.
func SummarizeMultipleCommandsSystemPrompt(username, loginName string) string {
	rootSuffix := ""
	if loginName == "root" {
		rootSuffix = prompts.RootPrompt
	}

	return fmt.Sprintf(
		"You are preparing a summary of %q's terminal session (logged in as %q) for security review.\n\n%s%s",
		username,
		loginName,
		prompts.SummaryPrompt,
		rootSuffix,
	)
}
