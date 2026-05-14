// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package llm

import (
	"context"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/trace"
)

// Report reports LLM usage.
//
// TODO(gabrielcorado): revisit this.
func (h *Handler) Report(ctx context.Context, format types.LLMFormat, usage usageReport) error {
	h.cfg.Log.DebugContext(ctx, "reporting llm usage", "format", format, "input_quota", h.cfg.InputTokensQuotaPerFormat, "output_quota", h.cfg.OutputTokensQuotaPerFormat)
	inputTokensCount.WithLabelValues(
		teleport.ComponentLLM,
		format,
	).Add(float64(usage.InputTokens))
	outputTokensCount.WithLabelValues(
		teleport.ComponentLLM,
		format,
	).Add(float64(usage.OutputTokens))

	switch format {
	case types.LLMFormatAnthropic:
		h.anthropicInputTokensCount.Add(usage.InputTokens)
		h.anthropicOutputTokensCount.Add(usage.OutputTokens)
	case types.LLMFormatOpenAI:
		h.openAIInputTokensCount.Add(usage.InputTokens)
		h.openAIOutputTokensCount.Add(usage.OutputTokens)
	}

	return nil
}

const beamsSchedulerUrl = "https://scheduler.zoom.us/d/oo1y6hyo/meet-with-the-beams-team"

func buildQuotaExeceededBeamsMessage(formatMessage string) string {
	return "Your tenant exceeded Teleport Beams free trial LLM tokens quota for " + formatMessage + " " +
		"Extend your trial or discuss commercial options by setting up time with our team: " + beamsSchedulerUrl
}

// canServeRequest checks if the request can be served.
//
// TODO(gabrielcorado): revisit this.
func (h *Handler) canServeRequest(_ context.Context, llmReq common.LLMRequest) error {
	var (
		consumedInputTokens  int64
		consumedOutputTokens int64
		extraMessage         string
	)

	switch llmReq.Format {
	case types.LLMFormatAnthropic:
		consumedInputTokens = h.anthropicInputTokensCount.Load()
		consumedOutputTokens = h.anthropicOutputTokensCount.Load()
		extraMessage = "Anthropic models, including Claude Code usage."
	case types.LLMFormatOpenAI:
		consumedInputTokens = h.openAIInputTokensCount.Load()
		consumedOutputTokens = h.openAIOutputTokensCount.Load()
		extraMessage = "OpenAI models, including Codex usage."
	}

	if consumedInputTokens >= h.cfg.InputTokensQuotaPerFormat {
		return trace.LimitExceeded("%s", buildQuotaExeceededBeamsMessage(extraMessage))
	}

	if consumedOutputTokens >= h.cfg.OutputTokensQuotaPerFormat {
		return trace.LimitExceeded("%s", buildQuotaExeceededBeamsMessage(extraMessage))
	}

	return nil
}

// usageReport is the internal struct that contains information about request
// LLM resources usage.
type usageReport struct {
	InputTokens  int64
	OutputTokens int64
}

type usageReporter interface {
	Report(context.Context, types.LLMFormat, usageReport) error
}
