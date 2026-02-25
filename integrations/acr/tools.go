package acr

import (
	"context"
	"fmt"

	"github.com/gravitational/teleport/integrations/acr/llm"
)

// Classify takes raw audit log JSON and returns events grouped by
// account/region, instance, and issue type.
func (s *Service) Classify(ctx context.Context, auditLogsJSON string) (*ClassifyResult, error) {
	prompt := fmt.Sprintf(classifyUserPrompt, auditLogsJSON)
	opts := llm.CompletionOptions{
		SystemPrompt: classifySystemPrompt,
		Temperature:  0.1,
		MaxTokens:    2000,
	}

	var result ClassifyResult
	if err := s.llm.CompleteJSON(ctx, prompt, opts, &result); err != nil {
		return nil, fmt.Errorf("acr: classifying audit logs: %w", err)
	}
	return &result, nil
}
