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

package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	appAccesslist "github.com/gravitational/teleport/integrations/access/accesslist"
	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
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
	clock       clockwork.Clock
	clusterName string
	webProxyURL *url.URL
}

// onAfterResponseSlack resty error function for Slack
func onAfterResponseSlack(sink common.StatusSink) func(_ *resty.Client, resp *resty.Response) error {
	return func(_ *resty.Client, resp *resty.Response) error {
		status := common.StatusFromStatusCode(resp.StatusCode())
		defer func() {
			if sink == nil {
				return
			}

			// No context in scope, use background with a reasonable timeout
			ctx, cancel := context.WithTimeout(context.Background(), statusEmitTimeout)
			defer cancel()
			if err := sink.Emit(ctx, status); err != nil {
				logger.Get(ctx).ErrorContext(ctx, "Error while emitting plugin status", "error", err)
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

// SupportedApps are the apps supported by this bot.
func (b Bot) SupportedApps() []common.App {
	return []common.App{
		accessrequest.NewApp(b),
		appAccesslist.NewApp(b),
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

// SendReviewReminders will send a review reminder that an access list needs to be reviewed.
func (b Bot) SendReviewReminders(ctx context.Context, recipient common.Recipient, accessLists []*accesslist.AccessList) error {
	var blockItem []BlockItem

	if len(accessLists) > 1 {
		blockItem = b.slackAccessListBatchedReminderMsgSection(accessLists)
	} else if len(accessLists) == 1 {
		blockItem = b.slackAccessListReminderMsgSection(accessLists[0])
	}

	var result ChatMsgResponse
	_, err := b.client.NewRequest().
		SetContext(ctx).
		SetBody(Message{BaseMessage: BaseMessage{Channel: recipient.ID}, BlockItems: blockItem}).
		SetResult(&result).
		Post("chat.postMessage")
	return trace.Wrap(err)
}

// BroadcastAccessRequestMessage posts request info to Slack with action buttons.
func (b Bot) BroadcastAccessRequestMessage(ctx context.Context, recipients []common.Recipient, reqID string, reqData pd.AccessRequestData) (accessrequest.SentMessages, error) {
	var data accessrequest.SentMessages
	var errors []error

	// Fetch the user as a recipient. The user is expected to be an e-mail here, as should be
	// the case with most SSO setups.
	userRecipient, err := b.FetchRecipient(ctx, reqData.User)
	if err != nil {
		logger.Get(ctx).WarnContext(ctx, "Unable to find user in Slack, will not be able to notify", "user", reqData.User)
	}

	// Include the user in the list of recipients if it exists.
	allRecipients := make([]common.Recipient, len(recipients), len(recipients)+1)
	copy(allRecipients, recipients)
	if userRecipient != nil {
		allRecipients = append(allRecipients, *userRecipient)
	}

	for _, recipient := range allRecipients {
		var result ChatMsgResponse
		_, err := b.client.NewRequest().
			SetContext(ctx).
			SetBody(Message{BaseMessage: BaseMessage{Channel: recipient.ID}, BlockItems: b.slackAccessRequestMsgSections(reqID, reqData)}).
			SetResult(&result).
			Post("chat.postMessage")
		if err != nil {
			errors = append(errors, trace.Wrap(err))
			continue
		}
		data = append(data, accessrequest.MessageData{ChannelID: result.Channel, MessageID: result.Timestamp})
	}

	return data, trace.NewAggregate(errors...)
}

func (b Bot) PostReviewReply(ctx context.Context, channelID, timestamp string, review types.AccessReview) error {
	text, err := accessrequest.MsgReview(review)
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

// NotifyUser directly messages a user when their request is updated
func (b Bot) NotifyUser(ctx context.Context, reqID string, reqData pd.AccessRequestData) error {
	recipient, err := b.FetchRecipient(ctx, reqData.User)
	if err != nil {
		return trace.Wrap(err)
	}

	if recipient.Kind != RecipientKindEmail {
		return trace.BadParameter("user was not found, cant directly notify")
	}

	_, err = b.client.NewRequest().
		SetContext(ctx).
		SetBody(Message{BaseMessage: BaseMessage{Channel: recipient.ID}, BlockItems: []BlockItem{
			NewBlockItem(SectionBlock{
				Text: NewTextObjectItem(MarkdownObject{Text: fmt.Sprintf("Request with ID %q has been updated: *%s*", reqID, reqData.ResolutionTag)}),
			}),
		}}).
		Post("chat.postMessage")
	return trace.Wrap(err)
}

// Expire updates request's Slack post with EXPIRED status and removes action buttons.
func (b Bot) UpdateMessages(ctx context.Context, reqID string, reqData pd.AccessRequestData, slackData accessrequest.SentMessages, reviews []types.AccessReview) error {
	var errors []error
	for _, msg := range slackData {
		_, err := b.client.NewRequest().
			SetContext(ctx).
			SetBody(Message{BaseMessage: BaseMessage{
				Channel:   msg.ChannelID,
				Timestamp: msg.MessageID,
			}, BlockItems: b.slackAccessRequestMsgSections(reqID, reqData)}).
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

const (
	RecipientKindEmail   = "Email"
	RecipientKindChannel = "Channel"
)

func (b Bot) FetchRecipient(ctx context.Context, name string) (*common.Recipient, error) {
	if lib.IsEmail(name) {
		channel, err := b.lookupDirectChannelByEmail(ctx, name)
		if err != nil {
			if err.Error() == "users_not_found" {
				return nil, trace.NotFound("email recipient '%s' not found: %s", name, err)
			}
			return nil, trace.Errorf("error resolving email recipient %s: %s", name, err)
		}
		return &common.Recipient{
			Name: name,
			ID:   channel,
			Kind: RecipientKindEmail,
			Data: nil,
		}, nil
	}
	// TODO: check if channel exists ?
	return &common.Recipient{
		Name: name,
		ID:   name,
		Kind: RecipientKindChannel,
		Data: nil,
	}, nil
}

// FetchOncallUsers fetches on-call users filtered by the provided annotations.
func (b Bot) FetchOncallUsers(ctx context.Context, req types.AccessRequest) ([]string, error) {
	return nil, trace.NotImplemented("fetch oncall users not implemented for plugin")
}

// slackAccessListReminderMsgSection builds an access list reminder Slack message section (obeys markdown).
func (b Bot) slackAccessListReminderMsgSection(accessList *accesslist.AccessList) []BlockItem {
	nextAuditDate := accessList.Spec.Audit.NextAuditDate

	link := ""
	if b.webProxyURL != nil {
		reqURL := *b.webProxyURL
		reqURL.Path = lib.BuildURLPath("web", "accesslists", accessList.Metadata.Name)
		reqURL.Fragment = "review"
		link = fmt.Sprintf("*Link*: %s", reqURL.String())
	}

	name := fmt.Sprintf("*%s*", accessList.Spec.Title)
	var msg string
	if b.clock.Now().After(nextAuditDate) {
		daysSinceDue := int(b.clock.Since(nextAuditDate).Hours() / 24)
		msg = fmt.Sprintf("Access List %s is %d day(s) past due for a review! Please review it.\n%s",
			name, daysSinceDue, link)
	} else {
		msg = fmt.Sprintf(
			"Access List %s is due for a review by %s. Please review it soon!\n%s",
			name, accessList.Spec.Audit.NextAuditDate.Format(time.DateOnly), link)
	}

	sections := []BlockItem{
		NewBlockItem(SectionBlock{
			Text: NewTextObjectItem(MarkdownObject{Text: msg}),
		}),
	}

	return sections
}

// slackAccessListReminderMsgSection builds an access list reminder Slack message section (obeys markdown).
func (b Bot) slackAccessListBatchedReminderMsgSection(accessLists []*accesslist.AccessList) []BlockItem {
	// Sort by earliest date due.
	slices.SortFunc(accessLists, func(a, b *accesslist.AccessList) int {
		return a.Spec.Audit.NextAuditDate.Compare(b.Spec.Audit.NextAuditDate)
	})

	accessList := accessLists[0]

	earliestNextAuditDate := accessList.Spec.Audit.NextAuditDate
	numOfReviewsRequired := len(accessLists)
	link := ""
	dueDate := ""

	if b.webProxyURL != nil {
		reqURL := *b.webProxyURL
		reqURL.Path = lib.BuildURLPath("web", "accesslists")
		link = fmt.Sprintf("*Link*: %s", reqURL.String())
	}

	if b.clock.Now().After(earliestNextAuditDate) {
		daysSinceDue := int(b.clock.Since(earliestNextAuditDate).Hours() / 24)
		dueDate = fmt.Sprintf("earliest of which is %d day(s) past due. Please review!",
			daysSinceDue)
	} else {
		dueDate = fmt.Sprintf(
			"earliest of which is due by %s. Please review them soon!",
			accessList.Spec.Audit.NextAuditDate.Format(time.DateOnly))
	}

	sections := []BlockItem{
		NewBlockItem(SectionBlock{
			Text: NewTextObjectItem(MarkdownObject{Text: fmt.Sprintf("%d Access Lists are due for reviews, %s\n%s", numOfReviewsRequired, dueDate, link)}),
		}),
	}

	return sections
}

// slackAccessRequestMsgSection builds an access request Slack message section (obeys markdown).
func (b Bot) slackAccessRequestMsgSections(reqID string, reqData pd.AccessRequestData) []BlockItem {
	fields := accessrequest.MsgFields(reqID, reqData, b.clusterName, b.webProxyURL)
	statusText := accessrequest.MsgStatusText(reqData.ResolutionTag, reqData.ResolutionReason)

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
