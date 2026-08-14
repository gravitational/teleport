package schema

import (
	"github.com/gravitational/teleport/e/lib/auth/summarizer/prompts"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema/schematypes"
)

type CommandAnalysis = schematypes.CommandAnalysis
type SessionAnalysis = schematypes.SessionAnalysis
type ProseEmbedding = schematypes.ProseEmbedding
type DesktopScreenshotAnalysis = schematypes.DesktopScreenshotAnalysis
type DesktopSessionEvent = schematypes.DesktopSessionEvent
type DesktopSessionAnalysis = schematypes.DesktopSessionAnalysis

// GetProseEmbedding returns the system prompt used to instruct the LLM to
// produce a dense prose embedding of a session.
func GetProseEmbedding() string {
	return prompts.ProserPrompt
}
