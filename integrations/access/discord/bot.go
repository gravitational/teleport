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

package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
)

const discordMaxConns = 100
const discordHTTPTimeout = 10 * time.Second
const discordRedColor = 13771309  // Green OxD2222D
const discordGreenColor = 2328611 // Red 0x2328611
const discordStatusUpdateTimeout = 10 * time.Second

// DiscordBot is a discord client that works with AccessRequest.
// It's responsible for formatting and posting a message on Discord when an
// action occurs with an access request: a new request popped up, or a
// request is processed/updated.
type DiscordBot struct {
	client      *resty.Client
	clusterName string
	webProxyURL *url.URL
}

// onAfterResponseDiscord creates and configures a post-response
// handler for Discord Requests. Handles routing status updates
// through to the status sink (if supplied).
func onAfterResponseDiscord(statusSink common.StatusSink) resty.ResponseMiddleware {
	return func(_ *resty.Client, resp *resty.Response) error {
		if statusSink != nil {
			emitStatusUpdate(resp, statusSink)
		}

		if resp.IsSuccess() {
			return nil
		}

		var result DiscordResponse
		if err := json.Unmarshal(resp.Body(), &result); err != nil {
			return trace.Wrap(err)
		}

		if result.Message != "" {
			return trace.Errorf("%s (code: %v, status: %d)", result.Message, result.Code, resp.StatusCode())
		}

		return trace.Errorf("Discord API returned error: %s (status: %d)", string(resp.Body()), resp.StatusCode())
	}
}

func emitStatusUpdate(resp *resty.Response, statusSink common.StatusSink) {
	status := common.StatusFromStatusCode(resp.StatusCode())

	// There is sensible context in scope for us to use when emitting the
	// status update. We can't use the context from the Resty response,
	// as that could already be canceled, which would block us from emitting
	// a status update showing that the plugin is currently broken.
	//
	// Using the background context with a reasonable timeout seems the
	// least-bad option.
	ctx, cancel := context.WithTimeout(context.Background(), discordStatusUpdateTimeout)
	defer cancel()

	if err := statusSink.Emit(ctx, status); err != nil {
		logger.Get(resp.Request.Context()).
			WithError(err).
			Errorf("Error while emitting Discord plugin status: %v", err)
	}
}

func (b DiscordBot) CheckHealth(ctx context.Context) error {
	_, err := b.client.NewRequest().
		SetContext(ctx).
		Get("/users/@me")
	if err != nil {
		return trace.Wrap(err, "health check failed, probably invalid token")
	}

	return nil
}

// SupportedApps are the apps supported by this bot.
func (b DiscordBot) SupportedApps() []common.App {
	return []common.App{
		accessrequest.NewApp(b),
	}
}

// SendReviewReminders will send a review reminder that an access list needs to be reviewed.
func (b DiscordBot) SendReviewReminders(ctx context.Context, recipients []common.Recipient, accessList *accesslist.AccessList) error {
	return trace.NotImplemented("access list review reminder is not yet implemented")
}

// BroadcastAccessRequestMessage posts request info to Discord.
func (b DiscordBot) BroadcastAccessRequestMessage(ctx context.Context, recipients []common.Recipient, reqID string, reqData pd.AccessRequestData) (accessrequest.SentMessages, error) {
	var data accessrequest.SentMessages
	var errors []error

	for _, recipient := range recipients {
		var result ChatMsgResponse
		_, err := b.client.NewRequest().
			SetContext(ctx).
			SetBody(DiscordMsg{
				Msg:  Msg{Channel: recipient.ID},
				Text: b.discordMsgText(reqID, reqData),
			}).
			SetResult(&result).
			Post("/channels/" + recipient.ID + "/messages")
		if err != nil {
			errors = append(errors, trace.Wrap(err))
			continue
		}
		data = append(data, accessrequest.MessageData{ChannelID: recipient.ID, MessageID: result.DiscordID})

	}

	return data, trace.NewAggregate(errors...)
}

// PostReviewReply does nothing as Discord does not have threaded replies
func (b DiscordBot) PostReviewReply(ctx context.Context, channelID, timestamp string, review types.AccessReview) error {
	return nil
}

// NotifyUser will send users a direct message with the access request status
func (b DiscordBot) NotifyUser(ctx context.Context, reqID string, reqData pd.AccessRequestData) error {
	return trace.NotImplemented("notify user not implemented for plugin")
}

// UpdateMessages updates already posted Discord messages
func (b DiscordBot) UpdateMessages(ctx context.Context, reqID string, reqData pd.AccessRequestData, messagingData accessrequest.SentMessages, reviews []types.AccessReview) error {
	var errors []error
	for _, msg := range messagingData {
		_, err := b.client.NewRequest().
			SetContext(ctx).
			SetBody(DiscordMsg{
				Msg:    Msg{Channel: msg.ChannelID},
				Text:   b.discordMsgText(reqID, reqData),
				Embeds: b.discordEmbeds(reviews),
			}).
			Patch("/channels/" + msg.ChannelID + "/messages/" + msg.MessageID)
		if err != nil {
			errors = append(errors, trace.Wrap(err))
		}
	}

	return trace.NewAggregate(errors...)
}

func (b DiscordBot) discordEmbeds(reviews []types.AccessReview) []DiscordEmbed {
	reviewEmbeds := make([]DiscordEmbed, len(reviews))
	for i, review := range reviews {
		if review.Reason != "" {
			review.Reason = lib.MarkdownEscape(review.Reason, accessrequest.ReviewReasonLimit)
		}

		var color int
		var titleFormat string
		switch review.ProposedState {
		case types.RequestState_APPROVED:
			color = discordGreenColor
			titleFormat = "Approved request at %s"
		case types.RequestState_DENIED:
			color = discordRedColor
			titleFormat = "Denied request at %s"
		}

		var description string
		if review.Reason != "" {
			description = fmt.Sprintf("Reason: %s", review.Reason)
		}

		reviewEmbeds[i] = DiscordEmbed{
			Title:       fmt.Sprintf(titleFormat, review.Created.Format(time.UnixDate)),
			Description: description,
			Color:       color,
			Author: struct {
				Name string `json:"name"`
			}{review.Author},
		}
	}
	return reviewEmbeds
}

func (b DiscordBot) discordMsgText(reqID string, reqData pd.AccessRequestData) string {
	return "You have a new Role Request:\n" +
		accessrequest.MsgFields(reqID, reqData, b.clusterName, b.webProxyURL) +
		accessrequest.MsgStatusText(reqData.ResolutionTag, reqData.ResolutionReason)
}

func (b DiscordBot) FetchRecipient(ctx context.Context, name string) (*common.Recipient, error) {
	// Discord does not support resolving email addresses with bot permissions
	// This bot does not implement channel name resolving yet, this is doable but will require caching
	// as the endpoint returns all channels at the same time and is rate-limited.
	// FetchRecipient currently only supports creating recipients from ChannelIDs.
	return &common.Recipient{
		Name: name,
		ID:   name,
		Kind: "Channel",
		Data: nil,
	}, nil
}
