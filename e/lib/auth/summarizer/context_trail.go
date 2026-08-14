package summarizer

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/gravitational/teleport/e/lib/auth/summarizer/tokenizer"
)

const (
	// maxContextTrailTokens is the hard cap on the total token count of the context trail prepended to each command's
	// prompt.
	maxContextTrailTokens = 1500
	// maxRecentBenignCommands is the number of most recent benign commands (risk score < riskScoreThreshold) retained
	// in the context trail.
	maxRecentBenignCommands = 5
	// riskScoreThreshold is the minimum risk score for a command to be considered "risky" and always retained in the
	// context trail.
	riskScoreThreshold = 30
)

// contextTrailEntry stores a pre-formatted prompt entry along with metadata used by the selection algorithm.
type contextTrailEntry struct {
	index      int
	riskScore  int
	riskLevel  string
	summary    string
	tokenCount int
	text       string
}

// contextTrail keeps prior command entries for prompt context.
type contextTrail struct {
	risky  []contextTrailEntry
	benign []contextTrailEntry
}

// add appends a new command entry to the context trail, categorizing it as risky or benign based on its risk score.
// It then evicts old entries if the combined token count exceeds the maxContextTrailTokens limit, prioritizing
// the retention of risky entries.
func (c *contextTrail) add(index int, entry *commandEntry) {
	riskScore := 0
	riskLevel := ""
	if entry.analysis != nil {
		riskScore = entry.analysis.RiskScore
		riskLevel = entry.analysis.RiskLevel
	}

	text := formatTrailEntry(index, entry.command, entry.shortDescription, riskLevel, riskScore)

	e := contextTrailEntry{
		index:      index,
		riskScore:  riskScore,
		riskLevel:  riskLevel,
		summary:    entry.shortDescription,
		tokenCount: tokenizer.CountTokens(text),
		text:       text,
	}

	if riskScore >= riskScoreThreshold {
		c.risky = append(c.risky, e)
	} else {
		c.benign = append(c.benign, e)

		if len(c.benign) > maxRecentBenignCommands {
			trimmed := make([]contextTrailEntry, maxRecentBenignCommands)

			copy(trimmed, c.benign[len(c.benign)-maxRecentBenignCommands:])

			c.benign = trimmed
		}
	}

	c.trim()
}

func (c *contextTrail) buildContextPrompt() string {
	selected := c.selectEntries()
	if len(selected) == 0 {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("PRIOR COMMAND CONTEXT:\n")
	sb.WriteString("The following commands were executed earlier in this session. Use this context to better assess risk patterns, attack chains, and suspicious sequences.\n")
	sb.WriteString("================\n")

	for _, e := range selected {
		sb.WriteString(e.text)
	}

	sb.WriteString("================\n\n")

	return sb.String()
}

// selectEntries combines risky and benign entries, sorts them by their original command index, and returns the result.
func (c *contextTrail) selectEntries() []contextTrailEntry {
	if len(c.risky) == 0 && len(c.benign) == 0 {
		return nil
	}

	selected := slices.Concat(c.risky, c.benign)
	slices.SortFunc(selected, func(a, b contextTrailEntry) int {
		return cmp.Compare(a.index, b.index)
	})

	return selected
}

// trim evicts entries until the combined token count fits within maxContextTrailTokens.
// It evicts benign entries first (oldest first), then risky entries (oldest first),
// always preserving at least the newest risky entry.
func (c *contextTrail) trim() {
	var total int
	for _, e := range c.risky {
		total += e.tokenCount
	}
	for _, e := range c.benign {
		total += e.tokenCount
	}

	if total <= maxContextTrailTokens {
		return
	}

	// Evict benign entries first (oldest first).
	var bi int
	for bi < len(c.benign) && total > maxContextTrailTokens {
		total -= c.benign[bi].tokenCount
		bi++
	}

	if bi > 0 {
		trimmed := make([]contextTrailEntry, len(c.benign)-bi)
		copy(trimmed, c.benign[bi:])
		c.benign = trimmed
	}

	if total <= maxContextTrailTokens {
		return
	}

	// Evict risky entries (oldest first), but always keep the newest one.
	maxEvictable := len(c.risky) - 1

	var ri int
	for ri < maxEvictable && total > maxContextTrailTokens {
		total -= c.risky[ri].tokenCount
		ri++
	}

	if ri > 0 {
		trimmed := make([]contextTrailEntry, len(c.risky)-ri)
		copy(trimmed, c.risky[ri:])
		c.risky = trimmed
	}

	// If the newest risky entry alone exceeds the budget, replace the raw command with a placeholder
	// to avoid resending potentially sensitive content (credentials, base64 payloads, etc.) while
	// preserving the risk metadata (summary, risk level, risk score).
	if total > maxContextTrailTokens && len(c.risky) > 0 {
		last := &c.risky[len(c.risky)-1]
		last.text = formatTrailEntry(
			last.index,
			"[TRUNCATED - command was unusually large, which may itself be a risk indicator]",
			last.summary,
			last.riskLevel,
			last.riskScore,
		)
		last.tokenCount = tokenizer.CountTokens(last.text)
	}
}

func formatTrailEntry(index int, command, summary, riskLevel string, riskScore int) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "%d. Command: %s\n", index, command)
	fmt.Fprintf(&sb, "   Summary: %s\n", summary)

	if riskLevel != "" {
		fmt.Fprintf(&sb, "   Risk Level: %s\n", riskLevel)
	}

	fmt.Fprintf(&sb, "   Risk Score: %d\n", riskScore)

	return sb.String()
}
