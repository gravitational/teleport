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
	"strings"

	"golang.org/x/term"
	"google.golang.org/protobuf/proto"

	prehogv1a "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
)

const (
	maxTeleportSkillLength = 64
	maxEncodedSize         = 256

	// Agents that use a Teleport skill are asked to set teleportSkillEnvVar to the skill
	// name.
	teleportSkillEnvVar = "TELEPORT_SKILL"
	teleportSkillPrefix = "teleport-"
)

// Skill names are expected to be only letters, numbers, dashes, such as teleport-discovery.
var teleportSkillName = regexp.MustCompile(`^[a-z0-9-]*$`)

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

// parse decodes and validates evidence from an untrusted source. Evidence is
// optional analytics data. parse returns nil for invalid or empty evidence.
func parse(encoded []byte) *prehogv1a.AgentUsageEvidence {
	evidence := decode(encoded)
	if evidence == nil || !isValid(evidence) || isEmpty(evidence) {
		return nil
	}
	return evidence
}

func isEmpty(evidence *prehogv1a.AgentUsageEvidence) bool {
	return evidence.GetTeleportSkill() == "" &&
		len(evidence.GetKnownAgentEnvironment()) == 0 &&
		len(evidence.GetKnownAgentParentProcess()) == 0 &&
		len(evidence.GetKnownAgentUserAgent()) == 0 &&
		!evidence.GetDeclaredWebDriver() &&
		!evidence.GetInteractiveTerminal()
}

func isValid(evidence *prehogv1a.AgentUsageEvidence) bool {
	skill := evidence.GetTeleportSkill()
	if len(skill) > maxTeleportSkillLength || !teleportSkillName.MatchString(skill) {
		return false
	}
	for _, tools := range [][]prehogv1a.AgentTool{
		evidence.GetKnownAgentEnvironment(),
		evidence.GetKnownAgentParentProcess(),
		evidence.GetKnownAgentUserAgent(),
	} {
		for _, tool := range tools {
			if tool == prehogv1a.AgentTool_AGENT_TOOL_UNSPECIFIED {
				return false
			}
			if _, ok := prehogv1a.AgentTool_name[int32(tool)]; !ok {
				return false
			}
		}
	}
	return true
}

// Collect reads agent usage evidence from environment variables and the
// terminal.
func Collect() *prehogv1a.AgentUsageEvidence {
	evidence := &prehogv1a.AgentUsageEvidence{
		TeleportSkill:         collectTeleportSkill(),
		KnownAgentEnvironment: collectEnvironment(),
		InteractiveTerminal:   term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())),
	}
	if isEmpty(evidence) {
		return nil
	}
	return evidence
}

func collectTeleportSkill() string {
	name := strings.TrimPrefix(strings.ToLower(os.Getenv(teleportSkillEnvVar)), teleportSkillPrefix)
	if len(name) > maxTeleportSkillLength || !teleportSkillName.MatchString(name) {
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
