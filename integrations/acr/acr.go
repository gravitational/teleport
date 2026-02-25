// Package acr implements discovery review analysis using LLM-powered
// insights over Teleport audit logs and configuration.
package acr

import (
	"fmt"
	"os"

	"github.com/gravitational/teleport/integrations/acr/llm"
)

// openAIKeyEnv is the environment variable for the OpenAI API key.
const openAIKeyEnv = "OPENAI_API_KEY"

// Issue represents a deduplicated error type on a single instance.
type Issue struct {
	Confidence   string `json:"confidence"`
	Count        int    `json:"count"`
	ErrorSummary string `json:"error_summary"`
	Remediation  string `json:"remediation"`
}

// Instance groups deduplicated issues for a single EC2 instance.
type Instance struct {
	InstanceID string  `json:"instance_id"`
	Issues     []Issue `json:"issues"`
}

// Account groups instances by AWS account and region.
type Account struct {
	AccountID string     `json:"account_id"`
	Region    string     `json:"region"`
	Instances []Instance `json:"instances"`
}

// ClassifyResult is the structured response from the LLM classification.
type ClassifyResult struct {
	Accounts    []Account `json:"accounts"`
	TotalEvents int       `json:"total_events"`
}

// Service orchestrates discovery review analysis.
type Service struct {
	llm *llm.Client
}

// NewService creates a new ACR service.
func NewService() (*Service, error) {
	apiKey := os.Getenv(openAIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("acr: %s environment variable is required", openAIKeyEnv)
	}
	client, err := llm.NewClient(apiKey)
	if err != nil {
		return nil, fmt.Errorf("acr: creating LLM client: %w", err)
	}
	return &Service{llm: client}, nil
}
