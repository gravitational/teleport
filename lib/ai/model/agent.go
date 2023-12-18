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

package model

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/ai/model/output"
	"github.com/gravitational/teleport/lib/ai/model/tools"
	"github.com/gravitational/teleport/lib/ai/tokens"
)

const (
	// The internal name used to create actions when the agent encounters an error, such as when parsing output.
	actionException = "_Exception"

	// The maximum amount of thought <-> observation iterations the agent is allowed to perform.
	maxIterations = 15

	// The maximum amount of time the agent is allowed to spend before yielding a final answer.
	maxElapsedTime = 5 * time.Minute

	// The special header the LLM has to respond with to indicate it's done.
	finalResponseHeader = "<FINAL RESPONSE>"
)

// NewAgent creates a new agent. The Assist agent which defines the model responsible for the Assist feature.
func NewAgent(toolCtx *tools.ToolContext, tools ...tools.Tool) *Agent {
	return &Agent{tools, toolCtx}
}

// Agent is a model storing static state which defines some properties of the chat model.
type Agent struct {
	tools   []tools.Tool
	toolCtx *tools.ToolContext
}

// AgentAction is an event type representing the decision to take a single action, typically a tool invocation.
type AgentAction struct {
	// The action to take, typically a tool name.
	Action string `json:"action"`

	// The input to the action, varies depending on the action.
	Input string `json:"input"`

	// The log is either a direct tool response or a thought prompt correlated to the input.
	Log string `json:"log"`

	// The reasoning is a string describing the reasoning behind the action.
	Reasoning string `json:"reasoning"`
}

// agentFinish is an event type representing the decision to finish a thought
// loop and return a final text answer to the user.
type agentFinish struct {
	// output must be Message, StreamingMessage, CompletionCommand, AccessRequest.
	output any
}

type executionState struct {
	llm               *openai.Client
	chatHistory       []openai.ChatCompletionMessage
	humanMessage      openai.ChatCompletionMessage
	intermediateSteps []AgentAction
	observations      []string
	tokenCount        *tokens.TokenCount
}

// PlanAndExecute runs the agent with a given input until it arrives at a text answer it is satisfied
// with or until it times out.
func (a *Agent) PlanAndExecute(ctx context.Context, llm *openai.Client, chatHistory []openai.ChatCompletionMessage, humanMessage openai.ChatCompletionMessage, progressUpdates func(*AgentAction)) (any, *tokens.TokenCount, error) {
	log.Trace("entering agent think loop")
	iterations := 0
	start := time.Now()
	tookTooLong := func() bool { return iterations > maxIterations || time.Since(start) > maxElapsedTime }
	state := &executionState{
		llm:               llm,
		chatHistory:       chatHistory,
		humanMessage:      humanMessage,
		intermediateSteps: make([]AgentAction, 0),
		observations:      make([]string, 0),
		tokenCount:        tokens.NewTokenCount(),
	}

	for {
		log.Tracef("performing iteration %v of loop, %v seconds elapsed", iterations, int(time.Since(start).Seconds()))

		// This is intentionally not context-based, as we want to finish the current step before exiting
		// and the concern is not that we're stuck but that we're taking too long over multiple iterations.
		if tookTooLong() {
			return nil, nil, trace.Errorf("timeout: agent took too long to finish")
		}

		output, err := a.takeNextStep(ctx, state, progressUpdates)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		if output.finish != nil {
			log.Tracef("agent finished with output: %#v", output.finish.output)

			return output.finish.output, state.tokenCount, nil
		}

		if output.action != nil {
			state.intermediateSteps = append(state.intermediateSteps, *output.action)
			state.observations = append(state.observations, output.observation)
		}

		iterations++
	}
}

func (a *Agent) DoAction(ctx context.Context, llm *openai.Client, action *AgentAction) (any, *tokens.TokenCount, error) {
	state := &executionState{
		llm:        llm,
		tokenCount: tokens.NewTokenCount(),
	}
	out, err := a.doAction(ctx, state, action)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	switch {
	case out.finish != nil:
		// If the tool already breaks execution, we don't have to do anything
		return out.finish.output, state.tokenCount, nil
	case out.observation != "":
		// If the tool doesn't break execution and returns a single observation,
		// we wrap the observation in a Message.
		return &output.Message{Content: out.observation}, state.tokenCount, nil
	default:
		return nil, state.tokenCount, trace.Errorf("action %s did not end execution nor returned an observation", action.Action)
	}
}

// stepOutput represents the inputs and outputs of a single thought step.
type stepOutput struct {
	// if the agent is done, finish is set.
	finish *agentFinish

	// if the agent is not done, action is set together with observation.
	action      *AgentAction
	observation string
}

func (e *executionState) isRepeatAction(action *AgentAction) bool {
	for _, previousAction := range e.intermediateSteps {
		if previousAction.Action == action.Action && previousAction.Input == action.Input {
			return true
		}
	}

	return false
}

func (a *Agent) takeNextStep(ctx context.Context, state *executionState, progressUpdates func(*AgentAction)) (stepOutput, error) {
	log.Trace("agent entering takeNextStep")
	defer log.Trace("agent exiting takeNextStep")

	action, finish, err := a.plan(ctx, state)
	if output.IsInvalidOutputError(err) {
		log.Tracef("agent encountered an invalid output error: %v, attempting to recover", err)
		action := &AgentAction{
			Action: actionException,
			Log:    "Invalid or incomplete response: " + err.Error(),
		}

		// The exception tool is currently a bit special, the observation is always equal to the input.
		// We can expand on this in the future to make it handle errors better.
		log.Tracef("agent decided on action %v and received observation %v", action.Action, action.Input)
		return stepOutput{action: action, observation: action.Log}, nil
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

	// we check against repeat actions to get the LLM out of confusion loops faster.
	if state.isRepeatAction(action) {
		return stepOutput{action: action, observation: "You've already ran this tool with this input."}, nil
	}

	// If action is set, the agent is not done and called upon a tool.
	progressUpdates(action)

	return a.doAction(ctx, state, action)
}

func (a *Agent) doAction(ctx context.Context, state *executionState, action *AgentAction) (stepOutput, error) {
	var tool tools.Tool
	for _, candidate := range a.tools {
		if candidate.Name() == action.Action {
			tool = candidate
			break
		}
	}

	if tool == nil {
		log.Tracef("agent picked an unknown tool %v", action.Action)
		action := &AgentAction{
			Action: actionException,
			Log:    fmt.Sprintf("No tool with name %s exists.", action.Action),
		}

		return stepOutput{action: action, observation: action.Log}, nil
	}

	// Here we switch on the tool type because even though all tools are presented as equal to the LLM
	// some are marked special and break the typical tool execution loop; those are handled here instead.
	switch tool := tool.(type) {
	case *tools.CommandExecutionTool:
		completion, err := tool.ParseInput(action.Input)
		if err != nil {
			action := &AgentAction{
				Action: actionException,
				Log:    "Invalid or incomplete response: " + err.Error(),
			}

			return stepOutput{action: action, observation: action.Log}, nil
		}

		log.Tracef("agent decided on command execution, let's translate to an agentFinish")
		return stepOutput{finish: &agentFinish{output: completion}}, nil
	case *tools.AccessRequestCreateTool:
		accessRequest, err := tool.ParseInput(action.Input)
		if err != nil {
			action := &AgentAction{
				Action: actionException,
				Log:    "Invalid or incomplete response: " + err.Error(),
			}

			return stepOutput{action: action, observation: action.Log}, nil
		}

		return stepOutput{finish: &agentFinish{output: accessRequest}}, nil
	case *tools.CommandGenerationTool:
		input, err := tool.ParseInput(action.Input)
		if err != nil {
			action := &AgentAction{
				Action: actionException,
				Log:    "Invalid or incomplete response: " + err.Error(),
			}

			return stepOutput{action: action, observation: action.Log}, nil
		}
		completion := &output.GeneratedCommand{
			Command: input.Command,
		}

		log.Tracef("agent decided on command generation, let's translate to an agentFinish")
		return stepOutput{finish: &agentFinish{output: completion}}, nil
	case *tools.AuditQueryGenerationTool:
		log.Tracef("Tool called with input:'%s'", action.Input)
		tableName, err := tool.ChooseEventTable(ctx, action.Input, state.tokenCount)
		// If the query was not answerable by audit logs,
		// we return to the agent thinking loop and tell that the tool failed
		if trace.IsNotFound(err) {
			return stepOutput{action: action, observation: err.Error()}, nil
		}
		if err != nil {
			return stepOutput{}, trace.Wrap(err)
		}

		log.Tracef("Tool chose to query table '%s'", tableName)
		response, err := tool.GenerateQuery(ctx, tableName, action.Input, state.tokenCount)
		if err != nil {
			return stepOutput{}, trace.Wrap(err)
		}

		return stepOutput{finish: &agentFinish{output: response}}, nil
	default:
		runOut, err := tool.Run(ctx, a.toolCtx, action.Input)
		if err != nil {
			return stepOutput{}, trace.Wrap(err)
		}
		return stepOutput{action: action, observation: runOut}, nil
	}
}

func (a *Agent) plan(ctx context.Context, state *executionState) (*AgentAction, *agentFinish, error) {
	scratchpad := a.constructScratchpad(state.intermediateSteps, state.observations)
	prompt := a.createPrompt(state.chatHistory, scratchpad, state.humanMessage)
	promptTokenCount, err := tokens.NewPromptTokenCounter(prompt)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	state.tokenCount.AddPromptCounter(promptTokenCount)

	stream, err := state.llm.CreateChatCompletionStream(
		ctx,
		openai.ChatCompletionRequest{
			Model:       openai.GPT432K,
			Messages:    prompt,
			Temperature: 0.3,
			Stream:      true,
		},
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	deltas := output.StreamToDeltas(stream)

	action, finish, completionTokenCounter, err := parsePlanningOutput(deltas)
	state.tokenCount.AddCompletionCounter(completionTokenCounter)
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

func (a *Agent) constructScratchpad(intermediateSteps []AgentAction, observations []string) []openai.ChatCompletionMessage {
	var thoughts []openai.ChatCompletionMessage
	for i, action := range intermediateSteps {
		if len(action.Reasoning) != 0 {
			thoughts = append(thoughts, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: action.Reasoning,
			})
		}

		thoughts = append(thoughts, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: conversationToolResponse(observations[i]),
		})
	}

	return thoughts
}

// PlanOutput describes the expected JSON output after asking it to plan its next action.
type PlanOutput struct {
	Action      string `json:"action"`
	ActionInput any    `json:"action_input"`
	Reasoning   string `json:"reasoning"`
}

// parsePlanningOutput parses the output of the model after asking it to plan its next action
// and returns the appropriate event type or an error.
func parsePlanningOutput(deltas <-chan string) (*AgentAction, *agentFinish, tokens.TokenCounter, error) {
	var text string
	for delta := range deltas {
		text += delta

		if strings.HasPrefix(text, finalResponseHeader) {
			message, tc, err := output.NewStreamingMessage(deltas, text, finalResponseHeader)
			if err != nil {
				return nil, nil, nil, trace.Wrap(err)
			}
			return nil, &agentFinish{output: message}, tc, nil
		}
	}

	completionTokenCount, err := tokens.NewSynchronousTokenCounter(text)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	log.Tracef("received planning output: \"%v\"", text)
	if outputString, found := strings.CutPrefix(text, finalResponseHeader); found {
		return nil, &agentFinish{output: &output.Message{Content: outputString}}, completionTokenCount, nil
	}

	response, err := output.ParseJSONFromModel[PlanOutput](text)
	if err != nil {
		log.WithError(err).Trace("failed to parse planning output")
		return nil, nil, nil, trace.Wrap(err)
	}

	if v, ok := response.ActionInput.(string); ok {
		return &AgentAction{Action: response.Action, Input: v}, nil, completionTokenCount, nil
	} else {
		input, err := json.Marshal(response.ActionInput)
		if err != nil {
			return nil, nil, nil, trace.Wrap(err)
		}

		return &AgentAction{Action: response.Action, Input: string(input), Reasoning: response.Reasoning}, nil, completionTokenCount, nil
	}
}
