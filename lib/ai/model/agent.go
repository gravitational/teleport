/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package model

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
)

const (
	actionFinalAnswer = "Final Answer"
	actionException   = "_Exception"
	maxIterations     = 15
	maxElapsedTime    = 5 * time.Minute
)

// NewAgent creates a new agent. The Assist agent which defines the model responsible for the Assist feature.
func NewAgent(assistClient assist.AssistEmbeddingServiceClient, username string) *Agent {
	return &Agent{
		tools: []Tool{
			&commandExecutionTool{},
			&embeddingRetrievalTool{
				assistClient: assistClient,
				currentUser:  username,
			},
		},
	}
}

// Agent is a model storing static state which defines some properties of the chat model.
type Agent struct {
	tools []Tool
}

// agentAction is an event type represetning the decision to take a single action, typically a tool invocation.
type agentAction struct {
	// The action to take, typically a tool name.
	action string

	// The input to the action, varies depending on the action.
	input string

	// The log is either a direct tool response or a thought prompt correlated to the input.
	log string
}

// agentFinish is an event type representing the decision to finish a thought
// loop and return a final text answer to the user.
type agentFinish struct {
	// output must be Message or CompletionCommand
	output any
}

type executionState struct {
	llm               *openai.Client
	chatHistory       []openai.ChatCompletionMessage
	humanMessage      openai.ChatCompletionMessage
	intermediateSteps []agentAction
	observations      []string
	tokensUsed        *TokensUsed
}

// PlanAndExecute runs the agent with a given input until it arrives at a text answer it is satisfied
// with or until it times out.
func (a *Agent) PlanAndExecute(ctx context.Context, llm *openai.Client, chatHistory []openai.ChatCompletionMessage, humanMessage openai.ChatCompletionMessage) (any, error) {
	log.Trace("entering agent think loop")
	iterations := 0
	start := time.Now()
	tookTooLong := func() bool { return iterations > maxIterations || time.Since(start) > maxElapsedTime }
	tokensUsed := newTokensUsed_Cl100kBase()
	state := &executionState{
		llm:               llm,
		chatHistory:       chatHistory,
		humanMessage:      humanMessage,
		intermediateSteps: make([]agentAction, 0),
		observations:      make([]string, 0),
		tokensUsed:        tokensUsed,
	}

	for {
		log.Tracef("performing iteration %v of loop, %v seconds elapsed", iterations, int(time.Since(start).Seconds()))

		// This is intentionally not context-based, as we want to finish the current step before exiting
		// and the concern is not that we're stuck but that we're taking too long over multiple iterations.
		if tookTooLong() {
			return nil, trace.Errorf("timeout: agent took too long to finish")
		}

		output, err := a.takeNextStep(ctx, state)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if output.finish != nil {
			log.Tracef("agent finished with output: %v", output.finish.output)
			switch v := output.finish.output.(type) {
			case *Message:
				v.TokensUsed = tokensUsed
				return v, nil
			case *CompletionCommand:
				v.TokensUsed = tokensUsed
				return v, nil
			default:
				return nil, trace.Errorf("invalid output type %T", v)
			}
		}

		if output.action != nil {
			state.intermediateSteps = append(state.intermediateSteps, *output.action)
			state.observations = append(state.observations, output.observation)
		}

		iterations++
	}
}

// stepOutput represents the inputs and outputs of a single thought step.
type stepOutput struct {
	// if the agent is done, finish is set.
	finish *agentFinish

	// if the agent is not done, action is set together with observation.
	action      *agentAction
	observation string
}

func (a *Agent) takeNextStep(ctx context.Context, state *executionState) (stepOutput, error) {
	log.Trace("agent entering takeNextStep")
	defer log.Trace("agent exiting takeNextStep")

	action, finish, err := a.plan(ctx, state)
	if err, ok := trace.Unwrap(err).(*invalidOutputError); ok {
		log.Tracef("agent encountered an invalid output error: %v, attempting to recover", err)
		action := &agentAction{
			action: actionException,
			input:  observationPrefix + "Invalid or incomplete response",
			log:    thoughtPrefix + err.Error(),
		}

		// The exception tool is currently a bit special, the observation is always equal to the input.
		// We can expand on this in the future to make it handle errors better.
		log.Tracef("agent decided on action %v and received observation %v", action.action, action.input)
		return stepOutput{action: action, observation: action.input}, nil
	}
	if err != nil {
		log.Tracef("agent encountered an error: %v", err)
		return stepOutput{}, trace.Wrap(err)
	}

	// If finish is set, the agent is done and did not call upon any tool.
	if finish != nil {
		log.Trace("agent picked finish, returning")
		return stepOutput{finish: finish}, nil
	}

	var tool Tool
	for _, candidate := range a.tools {
		if candidate.Name() == action.action {
			tool = candidate
			break
		}
	}

	if tool == nil {
		log.Tracef("agent picked an unknown tool %v", action.action)
		action := &agentAction{
			action: actionException,
			input:  observationPrefix + "Unknown tool",
			log:    thoughtPrefix + "No tool with name " + action.action + " exists.",
		}

		return stepOutput{action: action, observation: action.input}, nil
	}

	if tool, ok := tool.(*commandExecutionTool); ok {
		input, err := tool.parseInput(action.input)
		if err != nil {
			action := &agentAction{
				action: actionException,
				input:  observationPrefix + "Invalid or incomplete response",
				log:    thoughtPrefix + err.Error(),
			}

			return stepOutput{action: action, observation: action.input}, nil
		}

		completion := &CompletionCommand{
			Command: input.Command,
			Nodes:   input.Nodes,
			Labels:  input.Labels,
		}

		log.Tracef("agent decided on command execution, let's translate to an agentFinish")
		return stepOutput{finish: &agentFinish{output: completion}}, nil
	}

	output, err := tool.Run(ctx, action.input)
	if err != nil {
		return stepOutput{}, trace.Wrap(err)
	}
	output.action = action
	return *output, nil // TODO: probably wrong!
}

func (a *Agent) plan(ctx context.Context, state *executionState) (*agentAction, *agentFinish, error) {
	scratchpad := a.constructScratchpad(state.intermediateSteps, state.observations)
	prompt := a.createPrompt(state.chatHistory, scratchpad, state.humanMessage)
	resp, err := state.llm.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:    openai.GPT4,
			Messages: prompt,
		},
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	llmOut := resp.Choices[0].Message.Content
	err = state.tokensUsed.AddTokens(prompt, llmOut)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	action, finish, err := parsePlanningOutput(llmOut)
	return action, finish, trace.Wrap(err)
}

func (a *Agent) createPrompt(chatHistory, agentScratchpad []openai.ChatCompletionMessage, humanMessage openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	prompt := make([]openai.ChatCompletionMessage, 0)
	prompt = append(prompt, chatHistory...)
	toolList := strings.Builder{}
	toolNames := make([]string, 0, len(a.tools))
	for _, tool := range a.tools {
		toolNames = append(toolNames, tool.Name())
		toolList.WriteString("> ")
		toolList.WriteString(tool.Name())
		toolList.WriteString(": ")
		toolList.WriteString(tool.Description())
		toolList.WriteString("\n")
	}

	if len(a.tools) == 0 {
		toolList.WriteString("No tools available.")
	}

	formatInstructions := conversationParserFormatInstructionsPrompt(toolNames)
	newHumanMessage := conversationToolUsePrompt(toolList.String(), formatInstructions, humanMessage.Content)
	prompt = append(prompt, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: newHumanMessage,
	})

	prompt = append(prompt, agentScratchpad...)
	return prompt
}

func (a *Agent) constructScratchpad(intermediateSteps []agentAction, observations []string) []openai.ChatCompletionMessage {
	var thoughts []openai.ChatCompletionMessage
	for i, action := range intermediateSteps {
		thoughts = append(thoughts, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: action.log,
		}, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: conversationToolResponse(observations[i]),
		})
	}

	return thoughts
}

// parseJSONFromModel parses a JSON object from the model output and attempts to sanitize contaminant text
// to avoid triggering self-correction due to some natural language being bundled with the JSON.
// The output type is generic and thus the structure of the expected JSON varies depending on T.
func parseJSONFromModel[T any](text string) (T, *invalidOutputError) {
	cleaned := strings.TrimSpace(text)
	if strings.Contains(cleaned, "```json") {
		cleaned = strings.Split(cleaned, "```json")[1]
	}
	if strings.Contains(cleaned, "```") {
		cleaned = strings.Split(cleaned, "```")[0]
	}
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	var output T
	err := json.Unmarshal([]byte(cleaned), &output)
	if err != nil {
		return output, newInvalidOutputErrorWithParseError(err)
	}

	return output, nil
}

// planOutput describes the expected JSON output after asking it to plan it's next action.
type planOutput struct {
	Action       string `json:"action"`
	Action_input any    `json:"action_input"`
}

// parsePlanningOutput parses the output of the model after asking it to plan it's next action
// and returns the appropriate event type or an error.
func parsePlanningOutput(text string) (*agentAction, *agentFinish, error) {
	log.Tracef("received planning output: \"%v\"", text)
	response, err := parseJSONFromModel[planOutput](text)
	if err != nil {
		log.WithError(err).Trace("failed to parse planning output")
		return nil, nil, trace.Wrap(err)
	}

	if response.Action == actionFinalAnswer {
		outputString, ok := response.Action_input.(string)
		if !ok {
			return nil, nil, trace.Errorf("invalid final answer type %T", response.Action_input)
		}

		return nil, &agentFinish{output: &Message{Content: outputString}}, nil
	}

	if v, ok := response.Action_input.(string); ok {
		return &agentAction{action: response.Action, input: v}, nil, nil
	} else {
		input, err := json.Marshal(response.Action_input)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return &agentAction{action: response.Action, input: string(input)}, nil, nil
	}
}
