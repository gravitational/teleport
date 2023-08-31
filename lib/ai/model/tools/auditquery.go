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

package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"

	"github.com/gravitational/teleport/gen/go/eventschema"
	"github.com/gravitational/teleport/lib/ai/model/output"
	"github.com/gravitational/teleport/lib/ai/tokens"
)

type AuditQueryGenerationTool struct {
	LLM *openai.Client
}

func (t *AuditQueryGenerationTool) Name() string {
	return "Audit Query Generation"
}

func (t *AuditQueryGenerationTool) Description() string {
	return `Generates a SQL query that can be ran against teleport audit events.
The input must be a single string describing what the query must achieve.`
}

func (t *AuditQueryGenerationTool) Run(_ context.Context, _ *ToolContext, _ string) (string, error) {
	// This is stubbed because AuditQueryGenerationTool is handled specially.
	// This is because execution of this tool breaks the loop and returns a command suggestion to the user.
	// It is still handled as a tool because testing has shown that the LLM behaves better when it is treated as a tool.
	//
	// In addition, treating it as a Tool interface item simplifies the display and prompt assembly logic significantly.
	return "", trace.NotImplemented("not implemented")
}

// ChooseEventTable lists all supported events and uses the LLM as a zero shot
// classifier to find which event type can be used to answer the suer query.
func (t *AuditQueryGenerationTool) ChooseEventTable(ctx context.Context, input string, tc *tokens.TokenCount) (string, error) {
	tableList, err := eventschema.QueryableEventList()
	if err != nil {
		return "", trace.Wrap(err)
	}

	prompt := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: `Your job it to find the correct table to run a query on.
You will be given a list of tables, and a request from the user.
You MUST RESPOND ONLY with a single table name. If no table can answer the question, respond 'Cannot answer'.`,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: tableList,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: fmt.Sprintf("The user request is: %s", input),
		},
	}
	promptTokens, err := tokens.NewPromptTokenCounter(prompt)
	if err != nil {
		return "", trace.Wrap(err)
	}
	tc.AddPromptCounter(promptTokens)

	response, err := t.LLM.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:       openai.GPT4,
			Messages:    prompt,
			Temperature: 0,
		},
	)
	if err != nil {
		return "", trace.Wrap(err)
	}

	completion := response.Choices[0].Message.Content
	completionTokens, err := tokens.NewSynchronousTokenCounter(completion)
	if err != nil {
		return "", trace.Wrap(err)
	}
	tc.AddCompletionCounter(completionTokens)

	eventType := strings.Trim(strings.TrimSpace(strings.ToLower(completion)), "\"'.")
	if eventType == "cannot answer" {
		return "", trace.NotFound("No relevant event type found. The query cannot be answered by audit logs.")
	}
	if !eventschema.IsValidEventType(eventType) {
		return "", trace.CompareFailed("Model response is not a valid event type: '%s'", eventType)
	}

	return eventType, nil

}

// GenerateQuery takes an event type, fetches its schema, and calls the LLM to
// generate SQL and answer the user query.
func (t *AuditQueryGenerationTool) GenerateQuery(ctx context.Context, eventType, input string, tc *tokens.TokenCount) (*output.StreamingMessage, error) {
	eventSchema, err := eventschema.GetEventSchemaFromType(eventType)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tableSchema, err := eventSchema.TableSchema()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	prompt := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: fmt.Sprintf(`You are a tool that generates Athena SQL queries to inspect audit events.
You will be given the schema of a table and a user request.
You MUST RESPOND ONLY with an SQL query that answers the user request.
If the request cannot be answered, respond 'none'.
Today's date is DATE('%s')`, time.Now().Format("2006-01-02")),
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: fmt.Sprintf("The schema of the table `%s` is:\n\n%s", eventType, tableSchema),
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: fmt.Sprintf("The user request is: %s", input),
		},
	}
	promptTokens, err := tokens.NewPromptTokenCounter(prompt)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tc.AddPromptCounter(promptTokens)

	stream, err := t.LLM.CreateChatCompletionStream(
		ctx,
		openai.ChatCompletionRequest{
			Model:       openai.GPT4,
			Messages:    prompt,
			Temperature: 0,
			Stream:      true,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	deltas := output.StreamToDeltas(stream)
	message, completionTokens, err := output.NewStreamingMessage(deltas, "", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tc.AddCompletionCounter(completionTokens)

	return message, nil
}
