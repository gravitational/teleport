package summarizer

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
	"github.com/gravitational/teleport/e/lib/auth/summarizer/ttyterminal"
	"github.com/gravitational/teleport/lib/session"
)

type commandInferenceProvider interface {
	SummarizeCommand(ctx context.Context, sessionID session.ID, username, loginName, prompt string) (*schema.CommandAnalysis, error)
}

func summarizeReconstructedCommand(
	ctx context.Context,
	sessionID session.ID,
	provider commandInferenceProvider,
	pool *workerPool,
	cmd ttyterminal.Command,
	username,
	loginName,
	sessionMetadata,
	contextTrailPrompt string,
) (*schema.CommandAnalysis, error) {
	if cmd.ChunkCount() < 2 {
		prompt := sessionMetadata + contextTrailPrompt + cmd.PromptForChunk(0)

		response, err := provider.SummarizeCommand(ctx, sessionID, username, loginName, prompt)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return response, nil
	}

	cs := &commandSummarizer{
		cmd:                cmd,
		pool:               pool,
		provider:           provider,
		sessionID:          sessionID,
		username:           username,
		loginName:          loginName,
		sessionMetadata:    sessionMetadata,
		contextTrailPrompt: contextTrailPrompt,
	}

	responses, err := cs.summarizeCommandInChunks(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	synthesized, err := cs.synthesizeCommandChunks(ctx, responses)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return synthesized, nil
}

type commandSummarizer struct {
	cmd                ttyterminal.Command
	pool               *workerPool
	provider           commandInferenceProvider
	sessionID          session.ID
	username           string
	loginName          string
	sessionMetadata    string
	contextTrailPrompt string
}

type summarizedCommandResult struct {
	analysis *schema.CommandAnalysis
	error    error
}

func (c *commandSummarizer) summarizeCommandInChunks(ctx context.Context) ([]*summarizedCommandResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	numChunks := c.cmd.ChunkCount()

	responses := make([]*summarizedCommandResult, numChunks)

	var wg sync.WaitGroup

	for i := 0; i < numChunks; i++ {
		wg.Add(1)

		go func(chunkIndex int) {
			defer wg.Done()

			c.pool.acquire()
			defer c.pool.release()

			// Individual chunks get session metadata (so the LLM knows what server it's analyzing) but not the
			// context trail — these are parallel analyses of segments of a single command's output.
			// The trail is included in the synthesis step instead.
			prompt := c.sessionMetadata + c.cmd.PromptForChunk(chunkIndex)

			response, err := c.provider.SummarizeCommand(ctx, c.sessionID, c.username, c.loginName, prompt)

			responses[chunkIndex] = &summarizedCommandResult{
				analysis: response,
				error:    err,
			}
		}(i)
	}

	wg.Wait()

	return responses, nil
}

func (c *commandSummarizer) synthesizeCommandChunks(ctx context.Context, chunkResponses []*summarizedCommandResult) (*schema.CommandAnalysis, error) {
	synthesisPrompt := c.sessionMetadata + c.contextTrailPrompt + createSynthesisPrompt(chunkResponses)

	synthesizedResponse, err := c.provider.SummarizeCommand(ctx, c.sessionID, c.username, c.loginName, synthesisPrompt)
	if err != nil {
		return nil, trace.Wrap(err, "synthesizing command chunks")
	}

	return synthesizedResponse, nil
}

func createSynthesisPrompt(responses []*summarizedCommandResult) string {
	var sb strings.Builder

	sb.WriteString("You are analyzing a terminal command that was split into multiple chunks due to its length. ")
	sb.WriteString("Synthesize the following individual chunk analyses into a single comprehensive structured analysis.\n\n")

	fmt.Fprintf(&sb, "Synthesis of %d output chunks:\n", len(responses))
	sb.WriteString("================\n\n")

	var allSuspiciousFlags []string
	var allIOCs []string
	var anyPrivilegeEscalation bool
	var anyDataExfiltration bool
	var anyPersistence bool

	highestRiskLevel := "none"

	for i, resp := range responses {
		fmt.Fprintf(&sb, "Chunk %d/%d Analysis:\n", i+1, len(responses))
		if resp.error != nil {
			fmt.Fprintf(&sb, "[Error: %v]\n", resp.error)
		}

		if resp.analysis != nil {
			data := resp.analysis

			fmt.Fprintf(&sb, "- Command: %s\n", data.Command)
			fmt.Fprintf(&sb, "- Summary: %s\n", data.ShortDescription)
			fmt.Fprintf(&sb, "- Risk Level: %s\n", data.RiskLevel)
			fmt.Fprintf(&sb, "- Risk Score: %d\n", data.RiskScore)
			fmt.Fprintf(&sb, "- Success: %v\n", data.Success)

			allSuspiciousFlags = append(allSuspiciousFlags, data.SuspiciousFlags...)
			allIOCs = append(allIOCs, data.IOCs...)

			if data.PrivilegeEscalation {
				anyPrivilegeEscalation = true
			}
			if data.DataExfiltration {
				anyDataExfiltration = true
			}
			if data.Persistence {
				anyPersistence = true
			}

			if isHigherRisk(data.RiskLevel, highestRiskLevel) {
				highestRiskLevel = data.RiskLevel
			}

			if len(data.SuspiciousFlags) > 0 {
				fmt.Fprintf(&sb, "- Suspicious Flags: %v\n", data.SuspiciousFlags)
			}

			if data.HasSensitiveData {
				fmt.Fprintf(&sb, "- Sensitive Data: %v\n", data.SensitiveItems)
			}

			if len(data.IOCs) > 0 {
				fmt.Fprintf(&sb, "- IOCs: %v\n", data.IOCs)
			}
		}

		sb.WriteString("\n")
	}

	sb.WriteString("SYNTHESIS REQUIREMENTS:\n")
	sb.WriteString("1. Combine all chunk analyses into a single comprehensive structured analysis\n")
	sb.WriteString("2. The command field should show the complete command that was executed\n")
	sb.WriteString("3. Merge and deduplicate all findings (suspicious flags, IOCs, sensitive items, etc.)\n")
	sb.WriteString("4. Set the risk level to the highest risk detected across all chunks\n")
	sb.WriteString("5. Provide a comprehensive description that covers the entire command execution\n")
	sb.WriteString("6. Include ALL unique findings from ALL chunks in the appropriate fields\n\n")

	if len(allSuspiciousFlags) > 0 || len(allIOCs) > 0 || anyPrivilegeEscalation || anyDataExfiltration || anyPersistence {
		sb.WriteString("IMPORTANT FINDINGS TO INCLUDE:\n")
		if len(allSuspiciousFlags) > 0 {
			fmt.Fprintf(&sb, "- All Suspicious Flags: %v\n", utils.Deduplicate(allSuspiciousFlags))
		}
		if len(allIOCs) > 0 {
			fmt.Fprintf(&sb, "- All IOCs: %v\n", utils.Deduplicate(allIOCs))
		}
		if anyPrivilegeEscalation {
			sb.WriteString("- Privilege Escalation detected\n")
		}
		if anyDataExfiltration {
			sb.WriteString("- Data Exfiltration detected\n")
		}
		if anyPersistence {
			sb.WriteString("- Persistence mechanism detected\n")
		}
		fmt.Fprintf(&sb, "- Highest Risk Level: %s\n\n", highestRiskLevel)
	}

	return sb.String()
}

func isHigherRisk(risk1, risk2 string) bool {
	riskLevels := map[string]int{
		"none":     0,
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}

	level1 := riskLevels[risk1]
	level2 := riskLevels[risk2]

	return level1 > level2
}
