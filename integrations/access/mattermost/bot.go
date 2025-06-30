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

package mattermost

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/trace"
	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
)

const (
	mmMaxConns          = 100
	mmHTTPTimeout       = 10 * time.Second
	mmStatusEmitTimeout = 10 * time.Second
	mmCacheSize         = 1024
)

var postTextTemplate = template.Must(template.New("description").Parse(
	`{{if eq .Status "PENDING"}}*You have new pending request to review!*{{end}}
**User**: {{.User}}
**Roles**: {{range $index, $element := .Roles}}{{if $index}}, {{end}}{{ . }}{{end}}
**Request ID**: {{.ID}}
{{if .RequestReason}}**Reason**: {{.RequestReason}}{{end}}
**Status**: {{.StatusEmoji}} {{.Status}}
{{if .ResolutionReason}}**Resolution reason**: {{.ResolutionReason}}{{end}}
{{if .RequestLink}}**Link**: [{{.RequestLink}}]({{.RequestLink}})
{{else if eq .Status "PENDING"}}**Approve**: ` + "`tsh request review --approve {{.ID}}`" + `
**Deny**: ` + "`tsh request review --deny {{.ID}}`" + `{{end}}`,
))

var reviewCommentTemplate = template.Must(template.New("review comment").Parse(
	`{{.Author}} reviewed the request at {{.Created.Format .TimeFormat}}.
Resolution: {{.ProposedStateEmoji}} {{.ProposedState}}.
{{if .Reason}}Reason: {{.Reason}}.{{end}}`,
))

// Mattermost has a 4000 or 16k character limit for posts (depending on the
// configuration) so we truncate all reasons to a generous but conservative
// limit
const (
	requestReasonLimit = 500
	resolutionReasonLimit
	reviewReasonLimit
)

// Bot is a Mattermost client that works with access.Request.
type Bot struct {
	client      *resty.Client
	clusterName string
	webProxyURL *url.URL
}

type (
	getMeKey                       struct{}
	getChannelByTeamNameAndNameKey struct {
		team string
		name string
	}
)

type getUserByEmail struct {
	email string
}

type etagCacheCtxKey struct{}

type etagCacheEntry struct {
	etag  string
	value any
}

func NewBot(conf Config, clusterName, webProxyAddr string) (Bot, error) {
	var (
		webProxyURL *url.URL
		err         error
	)
	if webProxyAddr != "" {
		if webProxyURL, err = lib.AddrToURL(webProxyAddr); err != nil {
			return Bot{}, trace.Wrap(err)
		}
	}

	cache, err := lru.New[any, etagCacheEntry](mmCacheSize)
	if err != nil {
		return Bot{}, trace.Wrap(err, "failed to create cache")
	}

	client := resty.
		NewWithClient(&http.Client{
			Timeout: mmHTTPTimeout,
			Transport: &http.Transport{
				MaxConnsPerHost:     mmMaxConns,
				MaxIdleConnsPerHost: mmMaxConns,
				Proxy:               http.ProxyFromEnvironment,
			},
		}).
		SetBaseURL(conf.Mattermost.URL).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "BEARER "+conf.Mattermost.Token)

	// Error response parsing.
	client.OnBeforeRequest(func(_ *resty.Client, req *resty.Request) error {
		req.SetError(&ErrorResult{})
		return nil
	})
	client.OnAfterResponse(func(_ *resty.Client, resp *resty.Response) error {
		log := logger.Get(resp.Request.Context())

		status := common.StatusFromStatusCode(resp.StatusCode())
		sink := conf.StatusSink
		defer func() {
			if sink == nil {
				return
			}

			// No context in scope, use background with a reasonable timeout
			ctx, cancel := context.WithTimeout(context.Background(), mmStatusEmitTimeout)
			defer cancel()
			if err := sink.Emit(ctx, status); err != nil {
				log.ErrorContext(ctx, "Error while emitting plugin status", "error", err)
			}
		}()

		if !resp.IsError() {
			return nil
		}

		result := resp.Error()
		if result == nil {
			return nil
		}

		if result, ok := result.(*ErrorResult); ok {
			return trace.Wrap(result)
		}

		return trace.Errorf("unknown error result %#v", result)
	})

	// ETag caching.
	client.OnBeforeRequest(func(_ *resty.Client, req *resty.Request) error {
		if req.Method != resty.MethodGet {
			return nil
		}

		cacheKey := req.Context().Value(etagCacheCtxKey{})
		if cacheKey == nil {
			return nil
		}

		res, ok := cache.Get(cacheKey)
		if !ok {
			return nil
		}

		req.SetHeader("If-None-Match", res.etag)
		req.SetResult(res.value)
		return nil
	})
	client.OnAfterResponse(func(_ *resty.Client, resp *resty.Response) error {
		req := resp.Request
		if req.Method != resty.MethodGet {
			return nil
		}

		cacheKey := req.Context().Value(etagCacheCtxKey{})
		if cacheKey == nil {
			return nil
		}

		etag := resp.Header().Get("ETag")
		if etag == "" {
			return nil
		}

		if resp.IsSuccess() || resp.StatusCode() == http.StatusNotModified {
			cache.Add(cacheKey, etagCacheEntry{etag: etag, value: resp.Result()})
		}

		return nil
	})

	return Bot{
		client:      client,
		clusterName: clusterName,
		webProxyURL: webProxyURL,
	}, nil
}

// SupportedApps are the apps supported by this bot.
func (b Bot) SupportedApps() []common.App {
	return []common.App{
		accessrequest.NewApp(b),
	}
}

func (b Bot) CheckHealth(ctx context.Context) error {
	_, err := b.GetMe(ctx)
	return err
}

func (b Bot) GetMe(ctx context.Context) (User, error) {
	resp, err := b.client.NewRequest().
		SetContext(context.WithValue(ctx, etagCacheCtxKey{}, getMeKey{})).
		SetResult(&User{}).
		Get("api/v4/users/me")
	if err != nil {
		return User{}, trace.Wrap(err)
	}
	return userResult(resp)
}

// SendReviewReminders will send a review reminder that an access list needs to be reviewed.
func (b Bot) SendReviewReminders(ctx context.Context, recipients []common.Recipient, accessLists []*accesslist.AccessList) error {
	return trace.NotImplemented("access list review reminder is not yet implemented")
}

// BroadcastAccessRequestMessage posts request info to Mattermost.
func (b Bot) BroadcastAccessRequestMessage(ctx context.Context, recipients []common.Recipient, reqID string, reqData pd.AccessRequestData) (accessrequest.SentMessages, error) {
	text, err := b.buildPostText(reqID, reqData)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var data accessrequest.SentMessages
	var errors []error

	for _, recipient := range recipients {
		post := Post{
			ChannelID: recipient.ID,
			Message:   text,
		}
		_, err = b.client.NewRequest().
			SetContext(ctx).
			SetBody(post).
			SetResult(&post).
			Post("api/v4/posts")
		if err != nil {
			errors = append(errors, trace.Wrap(err))
			continue
		}

		data = append(data, accessrequest.MessageData{ChannelID: post.ChannelID, MessageID: post.ID})
	}

	return data, trace.NewAggregate(errors...)
}

func (b Bot) PostReviewReply(ctx context.Context, channelID, rootID string, review types.AccessReview) error {
	if review.Reason != "" {
		review.Reason = lib.MarkdownEscape(review.Reason, reviewReasonLimit)
	}

	var proposedStateEmoji string
	switch review.ProposedState {
	case types.RequestState_APPROVED:
		proposedStateEmoji = "✅"
	case types.RequestState_DENIED:
		proposedStateEmoji = "❌"
	}

	var builder strings.Builder
	err := reviewCommentTemplate.Execute(&builder, struct {
		types.AccessReview
		ProposedState      string
		ProposedStateEmoji string
		TimeFormat         string
	}{
		review,
		review.ProposedState.String(),
		proposedStateEmoji,
		time.RFC822,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	text := builder.String()

	_, err = b.client.NewRequest().
		SetContext(ctx).
		SetBody(Post{
			ChannelID: channelID,
			RootID:    rootID,
			Message:   text,
		}).
		Post("api/v4/posts")
	return trace.Wrap(err)
}

// LookupChannel fetches channel id by its name and team name.
func (b Bot) LookupChannel(ctx context.Context, team, name string) (string, error) {
	resp, err := b.client.NewRequest().
		SetContext(context.WithValue(ctx, etagCacheCtxKey{}, getChannelByTeamNameAndNameKey{team: team, name: name})).
		SetPathParams(map[string]string{"team": team, "name": name}).
		SetResult(&Channel{}).
		Get("api/v4/teams/name/{team}/channels/name/{name}")
	if err != nil {
		return "", trace.Wrap(err)
	}

	channel, err := channelResult(resp)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return channel.ID, nil
}

// LookupDirectChannel fetches user's direct message channel id by email.
func (b Bot) LookupDirectChannel(ctx context.Context, email string) (string, error) {
	resp, err := b.client.NewRequest().
		SetContext(context.WithValue(ctx, etagCacheCtxKey{}, getUserByEmail{email: email})).
		SetPathParams(map[string]string{"email": email}).
		SetResult(&User{}).
		Get("api/v4/users/email/{email}")
	if err != nil {
		return "", trace.Wrap(err)
	}
	user, err := userResult(resp)
	if err != nil {
		return "", trace.Wrap(err)
	}

	me, err := b.GetMe(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	resp, err = b.client.NewRequest().
		SetContext(ctx).
		SetBody([]string{me.ID, user.ID}).
		SetResult(&Channel{}).
		Post("api/v4/channels/direct")
	if err != nil {
		return "", trace.Wrap(err)
	}
	channel, err := channelResult(resp)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return channel.ID, nil
}

// NotifyUser will send users a direct message with the access request status
func (b Bot) NotifyUser(ctx context.Context, reqID string, reqData pd.AccessRequestData) error {
	return trace.NotImplemented("notify user not implemented for plugin")
}

func (b Bot) UpdateMessages(ctx context.Context, reqID string, reqData pd.AccessRequestData, mmData accessrequest.SentMessages, reviews []types.AccessReview) error {
	text, err := b.buildPostText(reqID, reqData)
	if err != nil {
		return trace.Wrap(err)
	}

	var errors []error
	for _, msg := range mmData {
		post := Post{
			ChannelID: msg.ChannelID,
			ID:        msg.MessageID,
			Message:   text,
		}
		_, err := b.client.NewRequest().
			SetContext(ctx).
			SetBody(post).
			SetPathParams(map[string]string{"postID": msg.MessageID}).
			Put("api/v4/posts/{postID}")
		if err != nil {
			errors = append(errors, trace.Wrap(err))
		}
	}

	return trace.NewAggregate(errors...)
}

func (b Bot) buildPostText(reqID string, reqData pd.AccessRequestData) (string, error) {
	resolutionTag := reqData.ResolutionTag

	if reqData.RequestReason != "" {
		reqData.RequestReason = lib.MarkdownEscape(reqData.RequestReason, requestReasonLimit)
	}
	if reqData.ResolutionReason != "" {
		reqData.ResolutionReason = lib.MarkdownEscape(reqData.ResolutionReason, resolutionReasonLimit)
	}

	var statusEmoji string
	status := string(resolutionTag)
	switch resolutionTag {
	case pd.Unresolved:
		status = "PENDING"
		statusEmoji = "⏳"
	case pd.ResolvedApproved:
		statusEmoji = "✅"
	case pd.ResolvedDenied:
		statusEmoji = "❌"
	case pd.ResolvedExpired:
		statusEmoji = "⌛"
	}

	var requestLink string
	if b.webProxyURL != nil {
		reqURL := *b.webProxyURL
		reqURL.Path = lib.BuildURLPath("web", "requests", reqID)
		requestLink = reqURL.String()
	}

	var (
		builder strings.Builder
		err     error
	)

	err = postTextTemplate.Execute(&builder, struct {
		ID          string
		Status      string
		StatusEmoji string
		RequestLink string
		pd.AccessRequestData
	}{
		reqID,
		status,
		statusEmoji,
		requestLink,
		reqData,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return builder.String(), nil
}

func (b Bot) tryLookupDirectChannel(ctx context.Context, userEmail string) string {
	log := logger.Get(ctx).With("mm_user_email", userEmail)
	channel, err := b.LookupDirectChannel(ctx, userEmail)
	if err != nil {
		var errResult *ErrorResult
		if errors.As(trace.Unwrap(err), &errResult) {
			log.WarnContext(ctx, "Failed to lookup direct channel info", "error", errResult.Message)
		} else {
			log.ErrorContext(ctx, "Failed to lookup direct channel info", "error", err)
		}
		return ""
	}
	return channel
}

func (b Bot) tryLookupChannel(ctx context.Context, team, name string) string {
	log := logger.Get(ctx).With(
		"mm_team", team,
		"mm_channel", name,
	)
	channel, err := b.LookupChannel(ctx, team, name)
	if err != nil {
		var errResult *ErrorResult
		if errors.As(trace.Unwrap(err), &errResult) {
			log.WarnContext(ctx, "Failed to lookup channel info", "error", errResult.Message)
		} else {
			log.ErrorContext(ctx, "Failed to lookup channel info", "error", err)
		}
		return ""
	}
	return channel
}

// FetchRecipient returns the recipient for the given raw recipient.
func (b Bot) FetchRecipient(ctx context.Context, name string) (*common.Recipient, error) {
	var channel string
	kind := "Channel"

	// Recipients from config file could contain either email or team and
	// channel names separated by '/' symbol. It's up to user what format to use.
	if lib.IsEmail(name) {
		channel = b.tryLookupDirectChannel(ctx, name)
		kind = "Email"
	} else {
		parts := strings.Split(name, "/")
		if len(parts) == 2 {
			channel = b.tryLookupChannel(ctx, parts[0], parts[1])
		} else {
			return nil, trace.BadParameter("Recipient must be either a user email or a channel in the format \"team/channel\" but got %q", name)
		}
	}

	return &common.Recipient{
		Name: name,
		ID:   channel,
		Kind: kind,
		Data: nil,
	}, nil
}

// FetchOncallUsers fetches on-call users filtered by the provided annotations.
func (b Bot) FetchOncallUsers(ctx context.Context, req types.AccessRequest) ([]string, error) {
	return nil, trace.NotImplemented("fetch oncall users not implemented for plugin")
}

func userResult(resp *resty.Response) (User, error) {
	result := resp.Result()
	ptr, ok := result.(*User)
	if !ok {
		return User{}, trace.Errorf("unknown result type %T", result)
	}
	return *ptr, nil
}

func channelResult(resp *resty.Response) (Channel, error) {
	result := resp.Result()
	ptr, ok := result.(*Channel)
	if !ok {
		return Channel{}, trace.Errorf("unknown result type %T", result)
	}
	return *ptr, nil
}
