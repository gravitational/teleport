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

package assist

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/jonboulle/clockwork"
	"github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/lib/ai"
	"github.com/gravitational/teleport/lib/ai/model"
)

// MessageType is a type of the Assist message.
type MessageType string

const (
	// MessageKindCommand is the type of Assist message that contains the command to execute.
	MessageKindCommand MessageType = "COMMAND"
	// MessageKindCommandResult is the type of Assist message that contains the command execution result.
	MessageKindCommandResult MessageType = "COMMAND_RESULT"
	// MessageKindCommandResultSummary is the type of message that is optionally
	// emitted after a command and contains a summary of the command output.
	// This message is both sent after the command execution to the web UI,
	// and persisted in the conversation history.
	MessageKindCommandResultSummary MessageType = "COMMAND_RESULT_SUMMARY"
	// MessageKindUserMessage is the type of Assist message that contains the user message.
	MessageKindUserMessage MessageType = "CHAT_MESSAGE_USER"
	// MessageKindAssistantMessage is the type of Assist message that contains the assistant message.
	MessageKindAssistantMessage MessageType = "CHAT_MESSAGE_ASSISTANT"
	// MessageKindAssistantPartialMessage is the type of Assist message that contains the assistant partial message.
	MessageKindAssistantPartialMessage MessageType = "CHAT_PARTIAL_MESSAGE_ASSISTANT"
	// MessageKindAssistantPartialFinalize is the type of Assist message that ends the partial message stream.
	MessageKindAssistantPartialFinalize MessageType = "CHAT_PARTIAL_MESSAGE_ASSISTANT_FINALIZE"
	// MessageKindSystemMessage is the type of Assist message that contains the system message.
	MessageKindSystemMessage MessageType = "CHAT_MESSAGE_SYSTEM"
	// MessageKindError is the type of Assist message that is presented to user as information, but not stored persistently in the conversation. This can include backend error messages and the like.
	MessageKindError MessageType = "CHAT_MESSAGE_ERROR"
)

// PluginGetter is the minimal interface used by the chat to interact with the plugin service in the backend.
type PluginGetter interface {
	PluginsClient() pluginsv1.PluginServiceClient
}

// MessageService is the minimal interface used by the chat to interact with the Assist message service in the backend.
type MessageService interface {
	// GetAssistantMessages returns all messages with given conversation ID.
	GetAssistantMessages(ctx context.Context, req *assist.GetAssistantMessagesRequest) (*assist.GetAssistantMessagesResponse, error)

	// CreateAssistantMessage adds the message to the backend.
	CreateAssistantMessage(ctx context.Context, msg *assist.CreateAssistantMessageRequest) error
}

// Assist is the Teleport Assist client.
type Assist struct {
	client *ai.Client
	// clock is a clock used to generate timestamps.
	clock clockwork.Clock
}

// NewClient creates a new Assist client.
func NewClient(ctx context.Context, proxyClient PluginGetter,
	proxySettings any, openaiCfg *openai.ClientConfig) (*Assist, error) {

	client, err := getAssistantClient(ctx, proxyClient, proxySettings, openaiCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Assist{
		client: client,
		clock:  clockwork.NewRealClock(),
	}, nil
}

// Chat is a Teleport Assist chat.
type Chat struct {
	assist *Assist
	chat   *ai.Chat
	// assistService is the auth server client.
	assistService MessageService
	// ConversationID is the ID of the conversation.
	ConversationID string
	// Username is the username of the user who started the chat.
	Username string
	// potentiallyStaleHistory indicates messages might have been inserted into
	// the chat history and the messages should be re-fetched before attempting
	// the next completion.
	potentiallyStaleHistory bool
}

// NewChat creates a new Assist chat.
func (a *Assist) NewChat(ctx context.Context, assistService MessageService,
	conversationID string, username string,
) (*Chat, error) {
	aichat := a.client.NewChat(authClient.EmbeddingClient(), username)

	chat := &Chat{
		assist:                  a,
		chat:                    aichat,
		assistService:           assistService,
		ConversationID:          conversationID,
		Username:                username,
		potentiallyStaleHistory: false,
	}

	if err := chat.loadMessages(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return chat, nil
}

// GenerateSummary generates a summary for the given message.
func (a *Assist) GenerateSummary(ctx context.Context, message string) (string, error) {
	return a.client.Summary(ctx, message)
}

// GenerateCommandSummary summarizes the output of a command executed on one or
// many nodes. The conversation history is also sent into the prompt in order
// to gather context and know what information is relevant in the command output.
func (a *Assist) GenerateCommandSummary(ctx context.Context, messages []*assist.AssistantMessage, output map[string][]byte) (string, error) {
	// Create system prompt
	modelMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: model.PromptSummarizeCommand},
	}

	// Load context back into prompt
	for _, message := range messages {
		role := kindToRole(MessageType(message.Type))
		if role != "" && role != openai.ChatMessageRoleSystem {
			payload, err := formatMessagePayload(message)
			if err != nil {
				return "", trace.Wrap(err)
			}
			modelMessages = append(modelMessages, openai.ChatCompletionMessage{Role: role, Content: payload})
		}
	}
	return a.client.CommandSummary(ctx, modelMessages, output)
}

// loadMessages loads the messages from the database.
func (c *Chat) loadMessages(ctx context.Context) error {
	// existing conversation, retrieve old messages
	messages, err := c.assistService.GetAssistantMessages(ctx, &assist.GetAssistantMessagesRequest{
		ConversationId: c.ConversationID,
		Username:       c.Username,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// restore conversation context.
	for _, msg := range messages.GetMessages() {
		role := kindToRole(MessageType(msg.Type))
		if role != "" {
			payload, err := formatMessagePayload(msg)
			if err != nil {
				return trace.Wrap(err)
			}
			c.chat.Insert(role, payload)
		}
	}

	return nil
}

// IsNewConversation returns true if the conversation has no messages yet.
func (c *Chat) IsNewConversation() bool {
	return len(c.chat.GetMessages()) == 1
}

// getAssistantClient returns the OpenAI client created base on Teleport Plugin information
// or the static token configured in YAML.
func getAssistantClient(ctx context.Context, proxyClient PluginGetter,
	proxySettings any, openaiCfg *openai.ClientConfig,
) (*ai.Client, error) {
	apiKey, err := getOpenAITokenFromDefaultPlugin(ctx, proxyClient)
	if err == nil {
		return ai.NewClient(apiKey), nil
	} else if !trace.IsNotFound(err) && !trace.IsNotImplemented(err) {
		// We ignore 2 types of errors here.
		// Unimplemented may be raised by the OSS server,
		// as PluginsService does not exist there yet.
		// NotFound means plugin does not exist,
		// in which case we should fall back on the static token configured in YAML.
		log.WithError(err).Error("Unexpected error fetching default OpenAI plugin")
	}

	// If the default plugin is not configured, try to get the token from the proxy settings.
	keyGetter, found := proxySettings.(interface{ GetOpenAIAPIKey() string })
	if !found {
		return nil, trace.Errorf("GetOpenAIAPIKey is not implemented on %T", proxySettings)
	}

	apiKey = keyGetter.GetOpenAIAPIKey()
	if apiKey == "" {
		return nil, trace.Errorf("OpenAI API key is not set")
	}

	// Allow using the passed config if passed.
	if openaiCfg != nil {
		return ai.NewClientFromConfig(*openaiCfg), nil
	}
	return ai.NewClient(apiKey), nil
}

// onMessageFunc is a function that is called when a message is received.
type onMessageFunc func(kind MessageType, payload []byte, createdTime time.Time) error

// ProcessComplete processes the completion request and returns the number of tokens used.
func (c *Chat) ProcessComplete(ctx context.Context, onMessage onMessageFunc, userInput string,
) (*model.TokensUsed, error) {
	var tokensUsed *model.TokensUsed

	// If data might have been inserted into the chat history, we want to
	// refresh and get the latest data before querying the model.
	if c.potentiallyStaleHistory {
		c.chat = c.assist.client.NewChat(c.Username)
		err := c.loadMessages(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// query the assistant and fetch an answer
	message, err := c.chat.Complete(ctx, userInput)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// write the user message to persistent storage and the chat structure
	c.chat.Insert(openai.ChatMessageRoleUser, userInput)

	// Do not write empty messages to the database.
	if userInput != "" {
		if err := c.assistService.CreateAssistantMessage(ctx, &assist.CreateAssistantMessageRequest{
			Message: &assist.AssistantMessage{
				Type:        string(MessageKindUserMessage),
				Payload:     userInput, // TODO(jakule): Sanitize the payload
				CreatedTime: timestamppb.New(c.assist.clock.Now().UTC()),
			},
			ConversationId: c.ConversationID,
			Username:       c.Username,
		}); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	switch message := message.(type) {
	case *model.Message:
		tokensUsed = message.TokensUsed
		c.chat.Insert(openai.ChatMessageRoleAssistant, message.Content)

		// write an assistant message to persistent storage
		protoMsg := &assist.CreateAssistantMessageRequest{
			ConversationId: c.ConversationID,
			Username:       c.Username,
			Message: &assist.AssistantMessage{
				Type:        string(MessageKindAssistantMessage),
				Payload:     message.Content,
				CreatedTime: timestamppb.New(c.assist.clock.Now().UTC()),
			},
		}

		if err := c.assistService.CreateAssistantMessage(ctx, protoMsg); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := onMessage(MessageKindAssistantMessage, []byte(message.Content), c.assist.clock.Now().UTC()); err != nil {
			return nil, trace.Wrap(err)
		}
	case *model.CompletionCommand:
		tokensUsed = message.TokensUsed
		payload := commandPayload{
			Command: message.Command,
			Nodes:   message.Nodes,
			Labels:  message.Labels,
		}

		payloadJson, err := json.Marshal(payload)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		msg := &assist.CreateAssistantMessageRequest{
			ConversationId: c.ConversationID,
			Username:       c.Username,
			Message: &assist.AssistantMessage{
				Type:        string(MessageKindCommand),
				Payload:     string(payloadJson),
				CreatedTime: timestamppb.New(c.assist.clock.Now().UTC()),
			},
		}

		if err := c.assistService.CreateAssistantMessage(ctx, msg); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := onMessage(MessageKindCommand, payloadJson, c.assist.clock.Now().UTC()); nil != err {
			return nil, trace.Wrap(err)
		}
		// As we emitted a command suggestion, the user might have run it. If
		// the command ran, a summary could have been inserted in the backend.
		// To take this command summary into account we note the history might
		// be stale.
		c.potentiallyStaleHistory = true
	default:
		return nil, trace.Errorf("unknown message type")
	}

	return tokensUsed, nil
}

func getOpenAITokenFromDefaultPlugin(ctx context.Context, proxyClient PluginGetter) (string, error) {
	// Try retrieving credentials from the plugin resource first
	openaiPlugin, err := proxyClient.PluginsClient().GetPlugin(ctx, &pluginsv1.GetPluginRequest{
		Name:        "openai-default",
		WithSecrets: true,
	})
	if err != nil {
		return "", trail.FromGRPC(err)
	}

	creds := openaiPlugin.Credentials.GetBearerToken()
	if creds == nil {
		return "", trace.BadParameter("malformed credentials")
	}

	return creds.Token, nil
}

// kindToRole converts a message kind to an OpenAI role.
func kindToRole(kind MessageType) string {
	switch kind {
	case MessageKindUserMessage:
		return openai.ChatMessageRoleUser
	case MessageKindAssistantMessage:
		return openai.ChatMessageRoleAssistant
	case MessageKindSystemMessage:
		return openai.ChatMessageRoleSystem
	case MessageKindCommandResultSummary:
		return openai.ChatMessageRoleUser
	default:
		return ""
	}
}

// formatMessagePayload generates the OpemAI message payload corresponding to
// an Assist message. Most Assist message payloads can be converted directly,
// but some payloads are JSON-formatted and must be processed before being
// passed to the model.
func formatMessagePayload(message *assist.AssistantMessage) (string, error) {
	switch MessageType(message.GetType()) {
	case MessageKindCommandResultSummary:
		var summary CommandExecSummary
		err := json.Unmarshal([]byte(message.GetPayload()), &summary)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return summary.String(), nil
	default:
		return message.GetPayload(), nil
	}
}
