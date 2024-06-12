/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ai/model"
	"github.com/gravitational/teleport/lib/ai/model/output"
	"github.com/gravitational/teleport/lib/ai/model/tools"
	"github.com/gravitational/teleport/lib/ai/testutils"
	"github.com/gravitational/teleport/lib/modules"
)

func TestChat_PromptTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		messages []openai.ChatCompletionMessage
		want     int
	}{
		{
			name:     "empty",
			messages: []openai.ChatCompletionMessage{},
			want:     0,
		},
		{
			name: "only system message",
			messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "Hello",
				},
			},
			want: 850,
		},
		{
			name: "system and user messages",
			messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "Hello",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Hi LLM.",
				},
			},
			want: 855,
		},
		{
			name: "tokenize our prompt",
			messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: model.PromptCharacter("Bob"),
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Show me free disk space on localhost node.",
				},
			},
			want: 1114,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			responses := []string{
				generateCommandResponse(t),
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")

				require.NotEmpty(t, responses, "Unexpected request")
				dataBytes := responses[0]
				_, err := w.Write([]byte(dataBytes))
				require.NoError(t, err, "Write error")

				responses = responses[1:]
			}))

			t.Cleanup(server.Close)

			cfg := openai.DefaultConfig("secret-test-token")
			cfg.BaseURL = server.URL + "/v1"

			client := NewClientFromConfig(cfg)

			toolContext := tools.ToolContext{
				User: "Bob",
			}
			chat := client.NewChat(&toolContext)

			for _, message := range tt.messages {
				chat.Insert(message.Role, message.Content)
			}

			ctx := context.Background()
			_, tokenCount, err := chat.Complete(ctx, "", func(aa *model.AgentAction) {})
			require.NoError(t, err)

			prompt, completion := tokenCount.CountAll()
			usedTokens := prompt + completion
			require.Equal(t, tt.want, usedTokens)
		})
	}
}

func TestChat_Complete(t *testing.T) {
	beforeModules := modules.GetModules()
	modules.SetModules(&modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
	})
	t.Cleanup(func() { modules.SetModules(beforeModules) })

	responses := [][]byte{
		[]byte(generateTextResponse()),
		[]byte(generateCommandResponse(t)),
		[]byte(generateAccessRequestResponse(t)),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		require.NotEmpty(t, responses, "Unexpected request")
		dataBytes := responses[0]

		_, err := w.Write(dataBytes)
		require.NoError(t, err, "Write error")

		responses = responses[1:]
	}))
	defer server.Close()

	cfg := openai.DefaultConfig("secret-test-token")
	cfg.BaseURL = server.URL + "/v1"
	client := NewClientFromConfig(cfg)

	toolContext := tools.ToolContext{
		User: "Bob",
	}
	chat := client.NewChat(&toolContext)

	ctx := context.Background()
	_, _, err := chat.Complete(ctx, "Hello", func(aa *model.AgentAction) {})
	require.NoError(t, err)

	chat.Insert(openai.ChatMessageRoleUser, "Show me free disk space on localhost node.")

	t.Run("text completion", func(t *testing.T) {
		msg, _, err := chat.Complete(ctx, "Show me free disk space", func(aa *model.AgentAction) {})
		require.NoError(t, err)

		require.IsType(t, &output.StreamingMessage{}, msg)
		streamingMessage := msg.(*output.StreamingMessage)
		require.Equal(t, "Which ", <-streamingMessage.Parts)
		require.Equal(t, "node do ", <-streamingMessage.Parts)
		require.Equal(t, "you want ", <-streamingMessage.Parts)
		require.Equal(t, "use?", <-streamingMessage.Parts)
	})

	t.Run("command completion", func(t *testing.T) {
		msg, _, err := chat.Complete(ctx, "localhost", func(aa *model.AgentAction) {})
		require.NoError(t, err)

		require.IsType(t, &output.CompletionCommand{}, msg)
		command := msg.(*output.CompletionCommand)
		require.Equal(t, "df -h", command.Command)
		require.Len(t, command.Nodes, 1)
		require.Equal(t, "localhost", command.Nodes[0])
	})

	t.Run("access request creation", func(t *testing.T) {
		msg, _, err := chat.Complete(ctx, "Now, request access to the resource with kind node, hostname Alpha.local and the Name a35161f0-a2dc-48e7-bdd2-49b81926cab7", func(aa *model.AgentAction) {})
		require.NoError(t, err)

		require.IsType(t, &output.AccessRequest{}, msg)
		request := msg.(*output.AccessRequest)
		require.Empty(t, request.Roles)
		require.Empty(t, request.SuggestedReviewers)
		require.Equal(t, "maintenance", request.Reason)
		require.Len(t, request.Resources, 1)
		require.Equal(t, "a35161f0-a2dc-48e7-bdd2-49b81926cab7", request.Resources[0].Name)
		require.Equal(t, "Alpha.local", request.Resources[0].FriendlyName)
	})
}

// generateTextResponse generates a response for a text completion
func generateTextResponse() string {
	dataBytes := []byte{}
	dataBytes = append(dataBytes, []byte("event: message\n")...)

	data := `{"id":"1","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":{"content": "<FINAL RESPONSE>Which ", "role": "assistant"}}]}`
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)
	dataBytes = append(dataBytes, []byte("event: message\n")...)

	data = `{"id":"2","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":{"content": "node do ", "role": "assistant"}}]}`
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)
	dataBytes = append(dataBytes, []byte("event: message\n")...)

	data = `{"id":"3","object":"completion","created":1598069255,"model":"gpt-4","choices":[{"index": 0, "delta":{"content": "you want ", "role": "assistant"}}]}`
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)
	dataBytes = append(dataBytes, []byte("event: message\n")...)

	data = `{"id":"4","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":{"content": "use?", "role": "assistant"}}]}`
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)
	dataBytes = append(dataBytes, []byte("event: done\n")...)

	dataBytes = append(dataBytes, []byte("data: [DONE]\n\n")...)

	return string(dataBytes)
}

// generateCommandResponse generates a response for the command "df -h" on the node "localhost"
func generateCommandResponse(t *testing.T) string {
	dataBytes := []byte{}
	dataBytes = append(dataBytes, []byte("event: message\n")...)

	actionObj := model.PlanOutput{
		Action: "Command Execution",
		ActionInput: struct {
			Command string   `json:"command"`
			Nodes   []string `json:"nodes"`
		}{"df -h", []string{"localhost"}},
	}
	actionJson, err := json.Marshal(actionObj)
	if err != nil {
		require.NoError(t, err)
	}

	obj := struct {
		Content string `json:"content"`
		Role    string `json:"role"`
	}{
		Content: string(actionJson),
		Role:    "assistant",
	}
	json, err := json.Marshal(obj)
	if err != nil {
		require.NoError(t, err)
	}

	data := fmt.Sprintf(`{"id":"1","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":%v}]}`, string(json))
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)

	dataBytes = append(dataBytes, []byte("event: done\n")...)
	dataBytes = append(dataBytes, []byte("data: [DONE]\n\n")...)

	return string(dataBytes)
}

func generateAccessRequestResponse(t *testing.T) string {
	dataBytes := []byte{}
	dataBytes = append(dataBytes, []byte("event: message\n")...)

	actionObj := model.PlanOutput{
		Action: "Create Access Requests",
		ActionInput: struct {
			SuggestedReviewers []string          `json:"suggested_reviewers"`
			Roles              []string          `json:"roles"`
			Resources          []output.Resource `json:"resources"`
			Reason             string            `json:"reason"`
		}{
			nil,
			nil,
			[]output.Resource{{Type: types.KindNode, Name: "a35161f0-a2dc-48e7-bdd2-49b81926cab7", FriendlyName: "Alpha.local"}},
			"maintenance",
		},
	}
	actionJson, err := json.Marshal(actionObj)
	if err != nil {
		require.NoError(t, err)
	}

	obj := struct {
		Content string `json:"content"`
		Role    string `json:"role"`
	}{
		Content: string(actionJson),
		Role:    "assistant",
	}
	json, err := json.Marshal(obj)
	if err != nil {
		require.NoError(t, err)
	}

	data := fmt.Sprintf(`{"id":"1","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":%v}]}`, string(json))
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)

	dataBytes = append(dataBytes, []byte("event: done\n")...)
	dataBytes = append(dataBytes, []byte("data: [DONE]\n\n")...)

	return string(dataBytes)
}

func TestChat_Complete_AuditQuery(t *testing.T) {
	// Test setup: generate the responses that will be served by our OpenAI mock
	action := model.PlanOutput{
		Action:      tools.AuditQueryGenerationToolName,
		ActionInput: "Lists user who connected to a server as root.",
		Reasoning:   "foo",
	}
	selectedAction, err := json.Marshal(action)
	require.NoError(t, err)
	const generatedQuery = "SELECT user FROM session_start WHERE login='root'"

	responses := []string{
		// The model must select the audit query tool
		string(selectedAction),
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

	// End of test setup, we run the agent
	chat := client.NewAuditQuery("bob")

	ctx := context.Background()
	// We insert a message to make the conversation not empty and skip the
	// greeting message.
	chat.Insert(openai.ChatMessageRoleUser, "Hello")
	result, _, err := chat.Complete(ctx, "List users who connected to a server as root", func(action *model.AgentAction) {})
	require.NoError(t, err)

	// We check that the agent returns the expected response
	message, ok := result.(*output.StreamingMessage)
	require.True(t, ok)
	require.Equal(t, generatedQuery, message.WaitAndConsume())
}
