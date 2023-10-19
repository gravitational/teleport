/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ai

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	assistpb "github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/lib/ai/model/output"
	"github.com/gravitational/teleport/lib/ai/model/tools"
	"github.com/gravitational/teleport/lib/ai/testutils"
)

func TestRunTool_AuditQueryGeneration(t *testing.T) {
	// Test setup: starting a mock openai server and creating the client
	const generatedQuery = "SELECT user FROM session_start WHERE login='root'"

	responses := []string{
		// Then the audit query tool chooses to request session.start events
		"session.start",
		// Finally the tool builds a query based on the provided schemas
		generatedQuery,
	}
	server := httptest.NewServer(testutils.GetTestHandlerFn(t, responses))
	t.Cleanup(server.Close)

	cfg := openai.DefaultConfig("secret-test-token")
	cfg.BaseURL = server.URL

	client := NewClientFromConfig(cfg)

	// Doing the test: Check that the AuditQueryGeneration tool can be invoked
	// through client.RunTool and validate its response.
	ctx := context.Background()
	toolCtx := &tools.ToolContext{User: "alice"}
	response, _, err := client.RunTool(ctx, toolCtx, tools.AuditQueryGenerationToolName, "List users who connected to a server as root")
	require.NoError(t, err)
	message, ok := response.(*output.StreamingMessage)
	require.True(t, ok)
	require.Equal(t, generatedQuery, message.WaitAndConsume())
}

type mockEmbeddingGetter struct {
	response []*assistpb.EmbeddedDocument
}

func (m *mockEmbeddingGetter) GetAssistantEmbeddings(ctx context.Context, in *assistpb.GetAssistantEmbeddingsRequest, opts ...grpc.CallOption) (*assistpb.GetAssistantEmbeddingsResponse, error) {
	return &assistpb.GetAssistantEmbeddingsResponse{Embeddings: m.response}, nil
}

func TestRunTool_EmbeddingRetrieval(t *testing.T) {
	// Test setup: starting a mock openai server and embedding getter,
	// then create the client.
	mock := &mockEmbeddingGetter{
		[]*assistpb.EmbeddedDocument{
			{
				Id:              "1",
				Content:         "foo",
				SimilarityScore: 1,
			},
			{
				Id:              "2",
				Content:         "bar",
				SimilarityScore: 0.9,
			},
		},
	}
	ctx := context.Background()
	toolCtx := &tools.ToolContext{AssistEmbeddingServiceClient: mock}

	responses := make([]string, 0)
	server := httptest.NewServer(testutils.GetTestHandlerFn(t, responses))
	t.Cleanup(server.Close)

	cfg := openai.DefaultConfig("secret-test-token")
	cfg.BaseURL = server.URL
	client := NewClientFromConfig(cfg)

	// Doing the test: Check that the EmbeddingRetrieval tool can be invoked
	// through client.RunTool and validate its response.
	input := tools.EmbeddingRetrievalToolInput{Question: "Find foobar"}
	inputText, err := json.Marshal(input)
	require.NoError(t, err)
	response, _, err := client.RunTool(ctx, toolCtx, "Nodes names and labels retrieval", string(inputText))
	require.NoError(t, err)
	message, ok := response.(*output.Message)
	require.True(t, ok)
	require.Equal(t, "foo\nbar\n", message.Content)
}
