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

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
)

const statusEmitTimeout = 10 * time.Second

// Bot is a slack client that works with AccessRequest.
// It's responsible for formatting and posting a message on Slack when an
// action occurs with an access request: a new request popped up, or a
// request is processed/updated.
type Bot struct {
	client      *slack.Client
	clusterName string
	webProxyURL *url.URL
}

// onAfterResponseSlack updates the status sink after a Slack response is received.
func onAfterResponseSlack(sink common.StatusSink, statusCode int, data []byte) error {
	status := common.StatusFromStatusCode(statusCode)
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

	// If we don't have a 200 response.
	if statusCode/100 != 2 {
		return trace.Errorf("slack api returned unexpected code %v", statusCode)
	}

	var result slack.SlackResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return trace.Wrap(err)
	}
	status = statusFromResponse(&result)

	if !result.Ok {
		return trace.Errorf("%s", result.Error)
	}

	return nil
}

func (b Bot) CheckHealth(ctx context.Context) error {
	_, err := b.client.AuthTestContext(ctx)
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
		channel, timestamp, err := b.client.PostMessageContext(ctx, recipient.ID,
			slack.MsgOptionBlocks(b.slackMsgSections(reqID, reqData)...))
		if err != nil {
			errors = append(errors, trace.Wrap(err))
			continue
		}
		data = append(data, common.MessageData{ChannelID: channel, MessageID: timestamp})
	}

	return data, trace.NewAggregate(errors...)
}

func (b Bot) PostReviewReply(ctx context.Context, channelID, timestamp string, review types.AccessReview) error {
	text, err := common.MsgReview(review)
	if err != nil {
		return trace.Wrap(err)
	}

	_, _, err = b.client.PostMessageContext(ctx, channelID, slack.MsgOptionTS(timestamp), slack.MsgOptionText(text, false))
	return trace.Wrap(err)
}

// LookupDirectChannelByEmail fetches user's id by email.
func (b Bot) lookupDirectChannelByEmail(ctx context.Context, email string) (string, error) {
	resp, err := b.client.GetUserByEmailContext(ctx, email)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return resp.ID, nil
}

// Expire updates request's Slack post with EXPIRED status and removes action buttons.
func (b Bot) UpdateMessages(ctx context.Context, reqID string, reqData pd.AccessRequestData, slackData common.SentMessages, reviews []types.AccessReview) error {
	var errors []error
	for _, msg := range slackData {
		_, _, _, err := b.client.UpdateMessageContext(ctx, msg.ChannelID, msg.MessageID,
			slack.MsgOptionBlocks(b.slackMsgSections(reqID, reqData)...))
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
func (b Bot) slackMsgSections(reqID string, reqData pd.AccessRequestData) []slack.Block {
	fields := common.MsgFields(reqID, reqData, b.clusterName, b.webProxyURL)
	statusText := common.MsgStatusText(reqData.ResolutionTag, reqData.ResolutionReason)

	sections := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, "You have a new Role Request:", false, false), nil, nil),
		slack.NewSectionBlock(slack.NewTextBlockObject(slack.MarkdownType, fields, false, false), nil, nil),
		slack.NewContextBlock("", slack.NewTextBlockObject(slack.MarkdownType, statusText, false, false)),
	}

	return sections
}
