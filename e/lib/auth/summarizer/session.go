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

const (
	maxTokensPerChunk       = 5000
	maxFinalSynthesisTokens = 5000
)

type sessionInferenceProvider interface {
	commandInferenceProvider
	SummarizeMultipleCommands(ctx context.Context, sessionID session.ID, username, loginName, prompt string) (*schema.SessionAnalysis, error)
}

type sessionAnalyzer struct {
	pool      *workerPool
	provider  sessionInferenceProvider
	sessionID session.ID
	username  string
	loginName string
}

func analyzeSessionCommands(
	ctx context.Context,
	sessionID session.ID,
	provider sessionInferenceProvider,
	pool *workerPool,
	commands <-chan ttyterminal.Command,
	username,
	loginName string,
) (*schema.SessionAnalysis, []*schema.CommandAnalysis, error) {
	sa := &sessionAnalyzer{
		pool:      pool,
		provider:  provider,
		sessionID: sessionID,
		username:  username,
		loginName: loginName,
	}

	return sa.analyzeCommands(ctx, commands)
}

type commandEntry struct {
	command          string
	shortDescription string
	analysis         *schema.CommandAnalysis
}

type commandChunk struct {
	entries    []*commandEntry
	tokenCount int
}

func (s *sessionAnalyzer) analyzeCommands(
	ctx context.Context,
	commands <-chan ttyterminal.Command,
) (*schema.SessionAnalysis, []*schema.CommandAnalysis, error) {
	var chunks []*commandChunk
	currentChunk := &commandChunk{}

	promptHeaderTokens := ttyterminal.CountTokens(chunkSynthesisPromptHeader(0))
	promptFooterTokens := ttyterminal.CountTokens(chunkSynthesisPromptFooter())

	var commandAnalyses []*schema.CommandAnalysis

	commandIndex := 0
	for cmd := range commands {
		commandIndex++

		result, err := summarizeReconstructedCommand(ctx, s.sessionID, s.provider, s.pool, cmd, s.username, s.loginName)
		if err != nil {
			return nil, nil, trace.Wrap(err, "summarizing command %d", commandIndex)
		}

		entry := &commandEntry{
			command:          result.Command,
			shortDescription: result.ShortDescription,
			analysis:         result,
		}

		result.StartOffset = cmd.StartOffset()
		result.EndOffset = cmd.EndOffset()

		commandAnalyses = append(commandAnalyses, result)

		entryText := formatEntryForPrompt(commandIndex, entry)
		entryTokens := ttyterminal.CountTokens(entryText)

		chunkTokens := promptHeaderTokens + currentChunk.tokenCount + entryTokens + promptFooterTokens
		if chunkTokens > maxTokensPerChunk && len(currentChunk.entries) > 0 {
			chunks = append(chunks, currentChunk)
			currentChunk = &commandChunk{}
		}

		currentChunk.entries = append(currentChunk.entries, entry)
		currentChunk.tokenCount += entryTokens
	}

	if len(currentChunk.entries) > 0 {
		chunks = append(chunks, currentChunk)
	}

	if len(chunks) == 0 {
		return nil, nil, trace.BadParameter("no commands to analyze")
	}

	if len(chunks) == 1 {
		analysis, err := s.synthesizeChunk(ctx, chunks[0])
		if err != nil {
			return nil, nil, trace.Wrap(err, "synthesizing single chunk")
		}

		return analysis, commandAnalyses, nil
	}

	chunkSummaries := s.summarizeChunksInParallel(ctx, chunks)

	analysis, err := s.synthesizeChunkSummaries(ctx, chunkSummaries)
	if err != nil {
		return nil, nil, trace.Wrap(err, "synthesizing chunk summaries")
	}

	return analysis, commandAnalyses, nil
}

func (s *sessionAnalyzer) synthesizeChunkSummaries(
	ctx context.Context,
	chunkSummaries []*summarizedSessionResult,
) (*schema.SessionAnalysis, error) {
	prompt := createFinalSynthesisPrompt(chunkSummaries)

	response, err := s.provider.SummarizeMultipleCommands(ctx, s.sessionID, s.username, s.loginName, prompt)
	if err != nil {
		return nil, trace.Wrap(err, "synthesizing final summary")
	}

	return response, nil
}

func (s *sessionAnalyzer) synthesizeChunk(
	ctx context.Context,
	chunk *commandChunk,
) (*schema.SessionAnalysis, error) {
	prompt := createChunkSynthesisPrompt(chunk)

	response, err := s.provider.SummarizeMultipleCommands(ctx, s.sessionID, s.username, s.loginName, prompt)
	if err != nil {
		return nil, trace.Wrap(err, "synthesizing chunk")
	}

	return response, nil
}

type summarizedSessionResult struct {
	analysis *schema.SessionAnalysis
	error    error
}

func (s *sessionAnalyzer) summarizeChunksInParallel(
	ctx context.Context,
	chunks []*commandChunk,
) []*summarizedSessionResult {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]*summarizedSessionResult, len(chunks))

	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Go(func() {
			s.pool.acquire()
			defer s.pool.release()

			result, err := s.synthesizeChunk(ctx, chunk)
			results[i] = &summarizedSessionResult{
				analysis: result,
				error:    err,
			}
		})
	}

	wg.Wait()

	return results
}

func formatEntryForPrompt(index int, entry *commandEntry) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "%d. Command: %s\n", index, entry.command)
	fmt.Fprintf(&sb, "   Summary: %s\n", entry.shortDescription)

	if entry.analysis != nil {
		fmt.Fprintf(&sb, "   Risk Level: %s\n", entry.analysis.RiskLevel)
		fmt.Fprintf(&sb, "   Risk Score: %d\n", entry.analysis.RiskScore)
		fmt.Fprintf(&sb, "   Threat Category: %s\n", entry.analysis.ThreatCategory)

		if len(entry.analysis.SuspiciousFlags) > 0 {
			fmt.Fprintf(&sb, "   Suspicious Flags: %v\n", entry.analysis.SuspiciousFlags)
		}
		if len(entry.analysis.SuspiciousPatterns) > 0 {
			fmt.Fprintf(&sb, "   Suspicious Patterns: %v\n", entry.analysis.SuspiciousPatterns)
		}
		if len(entry.analysis.IOCs) > 0 {
			fmt.Fprintf(&sb, "   IOCs: %v\n", entry.analysis.IOCs)
		}
		if len(entry.analysis.SensitiveItems) > 0 {
			fmt.Fprintf(&sb, "   Sensitive Items: %v\n", entry.analysis.SensitiveItems)
		}
		if len(entry.analysis.MitreAttackIDs) > 0 {
			//nolint:misspell // ignore MITRE
			fmt.Fprintf(&sb, "   MITRE ATT&CK IDs: %v\n", entry.analysis.MitreAttackIDs)
		}
		if entry.analysis.PrivilegeEscalation {
			sb.WriteString("   Privilege Escalation: true\n")
		}
		if entry.analysis.DataExfiltration {
			sb.WriteString("   Data Exfiltration: true\n")
		}
		if entry.analysis.Persistence {
			sb.WriteString("   Persistence: true\n")
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

func chunkSynthesisPromptHeader(entryCount int) string {
	var sb strings.Builder

	sb.WriteString("Synthesize the following command executions into a single comprehensive analysis.\n\n")
	fmt.Fprintf(&sb, "Commands in this chunk (%d total):\n", entryCount)
	sb.WriteString("================\n\n")

	return sb.String()
}

func chunkSynthesisPromptFooter() string {
	var sb strings.Builder

	sb.WriteString("SYNTHESIS REQUIREMENTS:\n")
	sb.WriteString("1. Provide an overall summary of what was accomplished in this sequence of commands\n")
	sb.WriteString("2. Set the risk level to the highest risk detected across all commands\n")
	sb.WriteString("3. Merge and deduplicate all suspicious flags, patterns, IOCs, and sensitive items\n")
	sb.WriteString("4. Identify any patterns or attack chains across commands\n")

	return sb.String()
}

func createChunkSynthesisPrompt(chunk *commandChunk) string {
	var sb strings.Builder

	sb.WriteString(chunkSynthesisPromptHeader(len(chunk.entries)))

	for i, entry := range chunk.entries {
		sb.WriteString(formatEntryForPrompt(i+1, entry))
	}

	sb.WriteString(chunkSynthesisPromptFooter())

	return sb.String()
}

func createFinalSynthesisPrompt(summaries []*summarizedSessionResult) string {
	var sb strings.Builder

	sb.WriteString("You are analyzing a terminal session that was split into multiple chunks. ")
	sb.WriteString("Synthesize the following chunk summaries into a single comprehensive session analysis.\n\n")

	fmt.Fprintf(&sb, "Session chunks (%d total):\n", len(summaries))
	sb.WriteString("================\n\n")

	var allSuspiciousActivities []string
	var allSecurityIncidents []string
	var anyCompromiseIndicators bool
	highestRiskLevel := "none"

	summaryTokenCount := 0
	truncated := false
	includedChunks := 0

	for i, s := range summaries {
		if s.error != nil {
			fmt.Fprintf(&sb, "Chunk %d/%d: [ERROR: %v]\n\n", i+1, len(summaries), s.error)

			continue
		}

		summary := s.analysis

		allSuspiciousActivities = append(allSuspiciousActivities, summary.SuspiciousActivities...)
		allSecurityIncidents = append(allSecurityIncidents, summary.SecurityIncidents...)

		if summary.CompromiseIndicators {
			anyCompromiseIndicators = true
		}

		if isHigherRisk(summary.RiskLevel, highestRiskLevel) {
			highestRiskLevel = summary.RiskLevel
		}

		if truncated {
			continue
		}

		chunkText := formatChunkSummary(i+1, len(summaries), summary)
		chunkTokens := ttyterminal.CountTokens(chunkText)

		if summaryTokenCount+chunkTokens > maxFinalSynthesisTokens {
			truncated = true
			fmt.Fprintf(&sb, "[TRUNCATED: %d additional chunks not shown due to size limits]\n\n", len(summaries)-i)
			continue
		}

		sb.WriteString(chunkText)
		summaryTokenCount += chunkTokens
		includedChunks++
	}

	if truncated {
		sb.WriteString("WARNING: This session exceeded analysis limits. ")
		fmt.Fprintf(&sb, "Only %d of %d chunks are shown above. ", includedChunks, len(summaries))
		sb.WriteString("Mark this session as too_large in your response.\n\n")
	}

	sb.WriteString("SYNTHESIS REQUIREMENTS:\n")
	sb.WriteString("1. Provide a comprehensive summary of the entire session\n")
	sb.WriteString("2. Identify the overall intent and outcome of the session\n")
	sb.WriteString("3. Set the risk level to the highest risk detected\n")
	sb.WriteString("4. Merge and deduplicate all findings\n")
	sb.WriteString("5. Identify any attack patterns or chains across the session\n")
	if truncated {
		sb.WriteString("6. Set too_large to true since this session was truncated\n")
	}
	sb.WriteString("\n")

	if len(allSuspiciousActivities) > 0 || len(allSecurityIncidents) > 0 || anyCompromiseIndicators {
		sb.WriteString("IMPORTANT FINDINGS TO INCLUDE:\n")
		if len(allSuspiciousActivities) > 0 {
			fmt.Fprintf(&sb, "- All Suspicious Activities: %v\n", utils.Deduplicate(allSuspiciousActivities))
		}
		if len(allSecurityIncidents) > 0 {
			fmt.Fprintf(&sb, "- All Security Incidents: %v\n", utils.Deduplicate(allSecurityIncidents))
		}
		if anyCompromiseIndicators {
			sb.WriteString("- Compromise Indicators detected\n")
		}
		fmt.Fprintf(&sb, "- Highest Risk Level: %s\n", highestRiskLevel)
	}

	return sb.String()
}

func formatChunkSummary(index, total int, summary *schema.SessionAnalysis) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Chunk %d/%d:\n", index, total)
	fmt.Fprintf(&sb, "- Summary: %s\n", summary.ShortDescription)
	fmt.Fprintf(&sb, "- Risk Level: %s\n", summary.RiskLevel)
	fmt.Fprintf(&sb, "- Risk Score: %d\n", summary.RiskScore)

	if len(summary.SuspiciousActivities) > 0 {
		fmt.Fprintf(&sb, "- Suspicious Activities: %v\n", summary.SuspiciousActivities)
	}
	if len(summary.SecurityIncidents) > 0 {
		fmt.Fprintf(&sb, "- Security Incidents: %v\n", summary.SecurityIncidents)
	}
	if summary.CompromiseIndicators {
		sb.WriteString("- Compromise Indicators: true\n")
	}

	sb.WriteString("\n")

	return sb.String()
}
