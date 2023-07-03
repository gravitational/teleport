/*
Copyright 2022 Gravitational, Inc.

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

package slack

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
)

const slackMaxConns = 100
const slackHTTPTimeout = 10 * time.Second
const statusEmitTimeout = 10 * time.Second

// Bot is a slack client that works with AccessRequest.
// It's responsible for formatting and posting a message on Slack when an
// action occurs with an access request: a new request popped up, or a
// request is processed/updated.
type Bot struct {
	client      *resty.Client
	clusterName string
	webProxyURL *url.URL
}

// onAfterResponseSlack resty error function for Slack
func onAfterResponseSlack(sink common.StatusSink) func(_ *resty.Client, resp *resty.Response) error {
	return func(_ *resty.Client, resp *resty.Response) error {
		status := statusFromStatusCode(resp.StatusCode())
		defer func() {
			if sink == nil {
				return
			}

			// No context in scope, use background with a reasonable timeout
			ctx, cancel := context.WithTimeout(context.Background(), statusEmitTimeout)
			defer cancel()
			if err := sink.Emit(ctx, status); err != nil {
				log.Errorf("Error while emitting plugin status: %v", err)
			}
		}()

		if !resp.IsSuccess() {
			return trace.Errorf("slack api returned unexpected code %v", resp.StatusCode())
		}

		var result APIResponse
		if err := json.Unmarshal(resp.Body(), &result); err != nil {
			return trace.Wrap(err)
		}
		status = statusFromResponse(&result)

		if !result.Ok {
			return trace.Errorf("%s", result.Error)
		}

		return nil
	}
}

func (b Bot) CheckHealth(ctx context.Context) error {
	_, err := b.client.NewRequest().
		SetContext(ctx).
		Post("auth.test")
	if err != nil {
		if err.Error() == "invalid_auth" {
			return trace.Wrap(err, "authentication failed, probably invalid token")
		}
		return trace.Wrap(err)
	}
	return nil
}

// Broadcast posts request info to Slack with action buttons.
func (b Bot) Broadcast(ctx context.Context, recipients []common.Recipient, reqID string, reqData pd.AccessRequestData) (common.SentMessages, error) {
	var data common.SentMessages
	var errors []error

	for _, recipient := range recipients {
		var result ChatMsgResponse
		_, err := b.client.NewRequest().
			SetContext(ctx).
			SetBody(Message{BaseMessage: BaseMessage{Channel: recipient.ID}, BlockItems: b.slackMsgSections(reqID, reqData)}).
			SetResult(&result).
			Post("chat.postMessage")
		if err != nil {
			errors = append(errors, trace.Wrap(err))
			continue
		}
		data = append(data, common.MessageData{ChannelID: result.Channel, MessageID: result.Timestamp})
	}

	return data, trace.NewAggregate(errors...)
}

func (b Bot) PostReviewReply(ctx context.Context, channelID, timestamp string, review types.AccessReview) error {
	text, err := common.MsgReview(review)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = b.client.NewRequest().
		SetContext(ctx).
		SetBody(Message{BaseMessage: BaseMessage{Channel: channelID, ThreadTs: timestamp}, Text: text}).
		Post("chat.postMessage")
	return trace.Wrap(err)
}

// LookupDirectChannelByEmail fetches user's id by email.
func (b Bot) lookupDirectChannelByEmail(ctx context.Context, email string) (string, error) {
	var result struct {
		APIResponse
		User User `json:"user"`
	}
	_, err := b.client.NewRequest().
		SetContext(ctx).
		SetQueryParam("email", email).
		SetResult(&result).
		Get("users.lookupByEmail")
	if err != nil {
		return "", trace.Wrap(err)
	}

	return result.User.ID, nil
}

// Expire updates request's Slack post with EXPIRED status and removes action buttons.
func (b Bot) UpdateMessages(ctx context.Context, reqID string, reqData pd.AccessRequestData, slackData common.SentMessages, reviews []types.AccessReview) error {
	var errors []error
	for _, msg := range slackData {
		_, err := b.client.NewRequest().
			SetContext(ctx).
			SetBody(Message{BaseMessage: BaseMessage{
				Channel:   msg.ChannelID,
				Timestamp: msg.MessageID,
			}, BlockItems: b.slackMsgSections(reqID, reqData)}).
			Post("chat.update")
		if err != nil {
			switch err.Error() {
			case "message_not_found":
				err = trace.Wrap(err, "cannot find message with timestamp %s in channel %s", msg.MessageID, msg.ChannelID)
			default:
				err = trace.Wrap(err)
			}
			errors = append(errors, trace.Wrap(err))
		}
	}

	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

func (b Bot) FetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error) {
	if lib.IsEmail(recipient) {
		channel, err := b.lookupDirectChannelByEmail(ctx, recipient)
		if err != nil {
			if err.Error() == "users_not_found" {
				return nil, trace.NotFound("email recipient '%s' not found: %s", recipient, err)
			}
			return nil, trace.Errorf("error resolving email recipient %s: %s", recipient, err)
		}
		return &common.Recipient{
			Name: recipient,
			ID:   channel,
			Kind: "Email",
			Data: nil,
		}, nil
	}
	// TODO: check if channel exists ?
	return &common.Recipient{
		Name: recipient,
		ID:   recipient,
		Kind: "Channel",
		Data: nil,
	}, nil
}

// msgSection builds a Slack message section (obeys markdown).
func (b Bot) slackMsgSections(reqID string, reqData pd.AccessRequestData) []BlockItem {
	fields := common.MsgFields(reqID, reqData, b.clusterName, b.webProxyURL)
	statusText := common.MsgStatusText(reqData.ResolutionTag, reqData.ResolutionReason)

	sections := []BlockItem{
		NewBlockItem(SectionBlock{
			Text: NewTextObjectItem(MarkdownObject{Text: "You have a new Role Request:"}),
		}),
		NewBlockItem(SectionBlock{
			Text: NewTextObjectItem(MarkdownObject{Text: fields}),
		}),
		NewBlockItem(ContextBlock{
			ElementItems: []ContextElementItem{
				NewContextElementItem(MarkdownObject{Text: statusText}),
			},
		}),
	}

	return sections
}
