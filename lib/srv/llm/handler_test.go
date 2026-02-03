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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

func TestConvertModelName(t *testing.T) {
	for name, tc := range map[string]struct {
		mappings     []*types.LLM_ModelMap
		defaultModel string
		reqModel     string
		expected     string
	}{
		"exact match": {
			mappings: []*types.LLM_ModelMap{
				{From: "claude-sonnet", To: "us.anthropic.claude-sonnet-v2:0"},
			},
			defaultModel: "us.anthropic.claude-default:0",
			reqModel:     "claude-sonnet",
			expected:     "us.anthropic.claude-sonnet-v2:0",
		},
		"case insensitive match": {
			mappings: []*types.LLM_ModelMap{
				{From: "claude-sonnet", To: "us.anthropic.claude-sonnet-v2:0"},
			},
			defaultModel: "us.anthropic.claude-default:0",
			reqModel:     "Claude-Sonnet",
			expected:     "us.anthropic.claude-sonnet-v2:0",
		},
		"leading and trailing whitespace trimmed": {
			mappings: []*types.LLM_ModelMap{
				{From: "claude-sonnet", To: "us.anthropic.claude-sonnet-v2:0"},
			},
			defaultModel: "us.anthropic.claude-default:0",
			reqModel:     "  claude-sonnet  ",
			expected:     "us.anthropic.claude-sonnet-v2:0",
		},
		"first matching mapping wins": {
			mappings: []*types.LLM_ModelMap{
				{From: "claude-sonnet", To: "first-match"},
				{From: "claude-sonnet", To: "second-match"},
			},
			defaultModel: "default",
			reqModel:     "claude-sonnet",
			expected:     "first-match",
		},
		"no match returns default model": {
			mappings: []*types.LLM_ModelMap{
				{From: "claude-opus", To: "us.anthropic.claude-opus:0"},
			},
			defaultModel: "us.anthropic.claude-default:0",
			reqModel:     "claude-haiku",
			expected:     "us.anthropic.claude-default:0",
		},
		"nil mappings returns default model": {
			mappings:     nil,
			defaultModel: "us.anthropic.claude-default:0",
			reqModel:     "claude-sonnet",
			expected:     "us.anthropic.claude-default:0",
		},
		"empty mappings returns default model": {
			mappings:     []*types.LLM_ModelMap{},
			defaultModel: "us.anthropic.claude-default:0",
			reqModel:     "claude-sonnet",
			expected:     "us.anthropic.claude-default:0",
		},
		"empty request model returns default": {
			mappings: []*types.LLM_ModelMap{
				{From: "claude-sonnet", To: "us.anthropic.claude-sonnet-v2:0"},
			},
			defaultModel: "us.anthropic.claude-default:0",
			reqModel:     "",
			expected:     "us.anthropic.claude-default:0",
		},
		"empty default model and no match returns empty": {
			mappings: []*types.LLM_ModelMap{
				{From: "claude-sonnet", To: "us.anthropic.claude-sonnet-v2:0"},
			},
			defaultModel: "",
			reqModel:     "unknown-model",
			expected:     "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			result := convertModelName(tc.mappings, tc.defaultModel, tc.reqModel)
			require.Equal(t, tc.expected, result)
		})
	}
}

// streamRecorder adapts an apievents.Stream to an events.SessionRecorder
// by adding a no-op io.Writer (required by the interface).
type streamRecorder struct {
	apievents.Stream
}

func (s *streamRecorder) Write(p []byte) (int, error) { return len(p), nil }

// newTestAudit creates a common.Audit that calls onRecord for each recorded event.
func newTestAudit(t *testing.T, onRecord func(apievents.PreparedSessionEvent)) common.Audit {
	t.Helper()
	streamer, err := events.NewCallbackStreamer(events.CallbackStreamerConfig{
		Inner: events.NewDiscardStreamer(),
		OnRecordEvent: func(_ context.Context, _ session.ID, pe apievents.PreparedSessionEvent) error {
			onRecord(pe)
			return nil
		},
	})
	require.NoError(t, err)
	stream, err := streamer.CreateAuditStream(t.Context(), "test-session")
	require.NoError(t, err)
	audit, err := common.NewAudit(common.AuditConfig{
		Emitter:  events.NewDiscardEmitter(),
		Recorder: events.WithNoOpPreparer(&streamRecorder{stream}),
	})
	require.NoError(t, err)
	return audit
}

// newTestApp creates a types.Application configured with the given LLM format and provider.
func newTestApp(t *testing.T, format types.LLM_Format, provider types.LLM_Provider) types.Application {
	t.Helper()
	app, err := types.NewAppV3(types.Metadata{
		Name: "test-llm-app",
	}, types.AppSpecV3{
		LLM: &types.LLM{
			Format:   format,
			Provider: provider,
		},
	})
	require.NoError(t, err)
	return app
}

// newTestSessionRequest creates an *http.Request with a SessionContext attached.
func newTestSessionRequest(t *testing.T, method, path string, body io.Reader, sessionCtx *common.SessionContext) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	return common.WithSessionContext(req, sessionCtx)
}

// newTestBedrockHandler creates a Handler configured for Bedrock tests.
// It uses a stub AWS config provider and overrides the Anthropic base URL.
func newTestBedrockHandler(t *testing.T, anthropicURL string) *Handler {
	t.Helper()
	h, err := NewHandler(t.Context(), HandlerConfig{
		Log: slog.Default(),
		AWSConfigProvider: awsconfig.ProviderFunc(func(ctx context.Context, region string, optFns ...awsconfig.OptionsFn) (aws.Config, error) {
			return aws.Config{
				Region:      "us-east-1",
				Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", "SESSION"),
			}, nil
		}),
		anthropicBaseURL: anthropicURL,
	})
	require.NoError(t, err)
	return h
}

// newTestHandler creates a Handler with SDK base URLs pointing at test servers.
func newTestHandler(t *testing.T, anthropicURL, openAIURL string) *Handler {
	t.Helper()
	h, err := NewHandler(t.Context(), HandlerConfig{
		Log:              slog.Default(),
		anthropicBaseURL: anthropicURL,
		openAIBaseURL:    openAIURL,
	})
	require.NoError(t, err)
	return h
}
