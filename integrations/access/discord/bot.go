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

package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/trace"
)

const discordMaxConns = 100
const discordHTTPTimeout = 10 * time.Second
const discordRedColor = 13771309  // Green OxD2222D
const discordGreenColor = 2328611 // Red 0x2328611

// DiscordBot is a discord client that works with AccessRequest.
// It's responsible for formatting and posting a message on Discord when an
// action occurs with an access request: a new request popped up, or a
// request is processed/updated.
type DiscordBot struct {
	client      *resty.Client
	clusterName string
	webProxyURL *url.URL
}

// onAfterResponseDiscord resty error function for Discord
func onAfterResponseDiscord(_ *resty.Client, resp *resty.Response) error {
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

func (b DiscordBot) CheckHealth(ctx context.Context) error {
	return nil
	resp, err := b.client.NewRequest().
		SetContext(ctx).
		Get("/oauth2/applications/@me")
		//Get("/oauth2/@me")
	if err != nil {
		return trace.Wrap(err, "health check failed, probably invalid token")
	}
	fmt.Println("=== HEALTH RESP ===", string(resp.Body()))

	return nil
}

// Broadcast posts request info to Discord.
func (b DiscordBot) Broadcast(ctx context.Context, recipients []common.Recipient, reqID string, reqData pd.AccessRequestData) (common.SentMessages, error) {
	var data common.SentMessages
	var errors []error

	for _, recipient := range recipients {
		fmt.Println("=== POSTING TO ===", "/channels/"+recipient.ID+"/messages")
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
		data = append(data, common.MessageData{ChannelID: recipient.ID, MessageID: result.DiscordID})

	}

	return data, trace.NewAggregate(errors...)
}

// PostReviewReply does nothing as Discord does not have threaded replies
func (b DiscordBot) PostReviewReply(ctx context.Context, channelID, timestamp string, review types.AccessReview) error {
	return nil
}

// UpdateMessages updates already posted Discord messages
func (b DiscordBot) UpdateMessages(ctx context.Context, reqID string, reqData pd.AccessRequestData, messagingData common.SentMessages, reviews []types.AccessReview) error {
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
			review.Reason = lib.MarkdownEscape(review.Reason, common.ReviewReasonLimit)
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
		common.MsgFields(reqID, reqData, b.clusterName, b.webProxyURL) +
		common.MsgStatusText(reqData.ResolutionTag, reqData.ResolutionReason)
}

func (b DiscordBot) FetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error) {
	// Discord does not support resolving email addresses with bot permissions
	// This bot does not implement channel name resolving yet, this is doable but will require caching
	// as the endpoint returns all channels at the same time and is rate-limited.
	// FetchRecipient currently only supports creating recipients from ChannelIDs.
	return &common.Recipient{
		Name: recipient,
		ID:   recipient,
		Kind: "Channel",
		Data: nil,
	}, nil
}
