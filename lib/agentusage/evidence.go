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

import (
	"context"
	"os"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/term"
	"google.golang.org/protobuf/proto"

	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
)

const (
	maxTeleportSkillLength = 64
	maxAIAgentLength       = 64
	maxEncodedSize         = 256

	// Agents that use a Teleport skill are asked to set teleportSkillEnvVar to the skill
	// name.
	teleportSkillEnvVar = "TELEPORT_SKILL"
	teleportSkillPrefix = "teleport-"

	// Agent tools set aiAgentEnvVar to their own name.
	aiAgentEnvVar = "AI_AGENT"
)

// Skill names are expected to be only letters, numbers, dashes, such as teleport-discovery.
var teleportSkillName = regexp.MustCompile(`^[a-z0-9-]*$`)

// AI_AGENT values are expected to be tool names such as pi or claude-code_2-1-290_agent.
// Unrecognized patterns are ignored to avoid collecting sensitive or bad data.
var aiAgentName = regexp.MustCompile(`^[A-Za-z0-9._-]*$`)

type contextKey struct{}

// ContextWithEvidence adds the AgentUsageEvidence to the given context.
func ContextWithEvidence(ctx context.Context, evidence *prehogv1a.AgentUsageEvidence) context.Context {
	if evidence == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, evidence)
}

// FromContext returns the evidence in ctx. Callers share one pointer. Do not
// modify the evidence.
func FromContext(ctx context.Context) *prehogv1a.AgentUsageEvidence {
	evidence, _ := ctx.Value(contextKey{}).(*prehogv1a.AgentUsageEvidence)
	return evidence
}

func encode(evidence *prehogv1a.AgentUsageEvidence) []byte {
	encoded, err := proto.Marshal(evidence)
	if err != nil || len(encoded) > maxEncodedSize {
		return nil
	}
	return encoded
}

func decode(encoded []byte) *prehogv1a.AgentUsageEvidence {
	if len(encoded) > maxEncodedSize {
		return nil
	}
	var evidence prehogv1a.AgentUsageEvidence
	if err := (proto.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(encoded, &evidence); err != nil {
		return nil
	}
	return &evidence
}

// parse reads evidence from an untrusted source. It returns nil when no usable
// evidence remains.
func parse(encoded []byte) *prehogv1a.AgentUsageEvidence {
	return sanitizeEvidence(decode(encoded))
}

// sanitizeEvidence removes unrecognized or invalid values from evidence.
func sanitizeEvidence(evidence *prehogv1a.AgentUsageEvidence) *prehogv1a.AgentUsageEvidence {
	if evidence == nil {
		return nil
	}
	if !validTeleportSkill(evidence.GetTeleportSkill()) {
		evidence.TeleportSkill = ""
	}
	if !validAIAgent(evidence.GetAiAgent()) {
		evidence.AiAgent = ""
	}
	evidence.KnownAgentEnvironment = knownTools(evidence.KnownAgentEnvironment)
	evidence.KnownAgentParentProcess = knownTools(evidence.KnownAgentParentProcess)
	evidence.KnownAgentUserAgent = knownTools(evidence.KnownAgentUserAgent)
	if isEmpty(evidence) {
		return nil
	}
	return evidence
}

func validTeleportSkill(name string) bool {
	return len(name) <= maxTeleportSkillLength && teleportSkillName.MatchString(name)
}

func validAIAgent(name string) bool {
	return len(name) <= maxAIAgentLength && aiAgentName.MatchString(name)
}

func knownTools(tools []prehogv1a.AgentTool) []prehogv1a.AgentTool {
	return slices.DeleteFunc(tools, func(tool prehogv1a.AgentTool) bool {
		_, known := prehogv1a.AgentTool_name[int32(tool)]
		return tool == prehogv1a.AgentTool_AGENT_TOOL_UNSPECIFIED || !known
	})
}

func isEmpty(evidence *prehogv1a.AgentUsageEvidence) bool {
	return evidence.GetTeleportSkill() == "" &&
		len(evidence.GetKnownAgentEnvironment()) == 0 &&
		len(evidence.GetKnownAgentParentProcess()) == 0 &&
		len(evidence.GetKnownAgentUserAgent()) == 0 &&
		!evidence.GetDeclaredWebDriver() &&
		!evidence.GetInteractiveTerminal() &&
		evidence.GetAiAgent() == ""
}

// Collect reads agent usage evidence from environment variables and the
// terminal.
func Collect() *prehogv1a.AgentUsageEvidence {
	evidence := &prehogv1a.AgentUsageEvidence{
		TeleportSkill:         collectTeleportSkill(),
		KnownAgentEnvironment: collectEnvironment(),
		InteractiveTerminal:   term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())),
		AiAgent:               collectAIAgent(),
	}
	if isEmpty(evidence) {
		return nil
	}
	return evidence
}

func collectTeleportSkill() string {
	name := strings.TrimPrefix(strings.ToLower(os.Getenv(teleportSkillEnvVar)), teleportSkillPrefix)
	if !validTeleportSkill(name) {
		return ""
	}
	return name
}

func collectAIAgent() string {
	name := os.Getenv(aiAgentEnvVar)
	if !validAIAgent(name) {
		return ""
	}
	return name
}

func collectEnvironment() []prehogv1a.AgentTool {
	var tools []prehogv1a.AgentTool
	for _, definition := range toolEvidenceDefinitions {
		for _, marker := range definition.environment {
			value := os.Getenv(marker.name)
			if value == "" || (marker.value != "" && value != marker.value) {
				continue
			}
			tools = append(tools, definition.tool)
			break
		}
	}
	return tools
}
