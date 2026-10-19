/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package agentusage

import prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"

type environmentMarker struct {
	name string
	// When value is empty, environment variables present with any value match
	value string
}

type toolEvidenceDefinition struct {
	tool              prehogv1a.AgentTool
	environment       []environmentMarker
	userAgentProducts []string
}

// toolEvidenceDefinitions lists tools in AgentTool order. Collectors return
// matches in this order. Each definition lists every supported signal for one
// tool.
var toolEvidenceDefinitions = []toolEvidenceDefinition{
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_CLAUDE_CODE,
		// https://code.claude.com/docs/en/env-vars
		environment: []environmentMarker{{name: "CLAUDECODE", value: "1"}},
	},
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_CODEX_CLI,
		// Codex CLI sets CODEX_CI to 1 and CODEX_THREAD_ID to the thread ID in
		// the environment of the commands it runs.
		environment: []environmentMarker{
			{name: "CODEX_CI", value: "1"},
			{name: "CODEX_THREAD_ID"},
		},
	},
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_GEMINI_CLI,
		// https://github.com/google-gemini/gemini-cli/blob/main/docs/tools/shell.md
		environment: []environmentMarker{{name: "GEMINI_CLI", value: "1"}},
	},
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_CURSOR_AGENT,
		// https://cursor.com/docs/agent/tools/terminal
		environment: []environmentMarker{{name: "CURSOR_AGENT"}},
	},
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_OPENCODE,
		// opencode added the marker for tool detection but does not document it.
		// https://github.com/anomalyco/opencode/issues/1775
		environment: []environmentMarker{{name: "OPENCODE", value: "1"}},
	},
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_CHATGPT,
		// https://platform.openai.com/docs/bots
		userAgentProducts: []string{"ChatGPT-User"},
	},
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_CLAUDE,
		// https://support.claude.com/en/articles/8896518-does-anthropic-crawl-data-from-the-web-and-how-can-site-owners-block-the-crawler
		userAgentProducts: []string{"Claude-User"},
	},
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_PERPLEXITY,
		// https://docs.perplexity.ai/guides/bots
		userAgentProducts: []string{"Perplexity-User"},
	},
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_GOOGLE_AGENT,
		// https://developers.google.com/crawling/docs/crawlers-fetchers/google-agent
		userAgentProducts: []string{"Google-Agent"},
	},
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_GEMINI_NOTEBOOK,
		// https://developers.google.com/crawling/docs/crawlers-fetchers/google-gemininotebook
		userAgentProducts: []string{"Google-GeminiNotebook"},
	},
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_ANTIGRAVITY,
		// The Antigravity CLI and app set ANTIGRAVITY_AGENT to 1 in the
		// environment of the commands they run.
		environment: []environmentMarker{{name: "ANTIGRAVITY_AGENT", value: "1"}},
	},
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_PI,
		// https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/environment-variables.md
		environment: []environmentMarker{
			{name: "AI_AGENT", value: "pi"},
			{name: "PI_CODING_AGENT", value: "true"},
		},
	},
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_CLINE,
		// Cline added the marker for tool detection but does not document it. It
		// is set only for commands Cline runs in a VS Code terminal.
		// https://github.com/cline/cline/discussions/5366
		environment: []environmentMarker{{name: "CLINE_ACTIVE", value: "true"}},
	},
	{
		tool: prehogv1a.AgentTool_AGENT_TOOL_GITHUB_COPILOT,
		// Copilot CLI sets COPILOT_CLI to 1 and COPILOT_AGENT_SESSION_ID to the
		// session ID. Copilot agent mode in VS Code sets AI_AGENT to
		// github_copilot_vscode_agent and COPILOT_AGENT to 1.
		environment: []environmentMarker{
			{name: "COPILOT_CLI", value: "1"},
			{name: "COPILOT_AGENT_SESSION_ID"},
			{name: "AI_AGENT", value: "github_copilot_vscode_agent"},
			{name: "COPILOT_AGENT", value: "1"},
		},
	},
}
