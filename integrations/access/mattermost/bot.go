// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mattermost

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/trace"
	"github.com/mailgun/holster/v3/collections"
)

const (
	mmMaxConns    = 100
	mmHTTPTimeout = 10 * time.Second
	mmCacheSize   = 1024
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

type getMeKey struct{}
type getChannelByTeamNameAndNameKey struct {
	team string
	name string
}
type getUserByEmail struct {
	email string
}

type etagCacheCtxKey struct{}

type etagCacheEntry struct {
	etag  string
	value interface{}
}

func NewBot(conf MattermostConfig, clusterName, webProxyAddr string) (Bot, error) {
	var (
		webProxyURL *url.URL
		err         error
	)
	if webProxyAddr != "" {
		if webProxyURL, err = lib.AddrToURL(webProxyAddr); err != nil {
			return Bot{}, trace.Wrap(err)
		}
	}

	cache := collections.NewLRUCache(mmCacheSize)

	client := resty.
		NewWithClient(&http.Client{
			Timeout: mmHTTPTimeout,
			Transport: &http.Transport{
				MaxConnsPerHost:     mmMaxConns,
				MaxIdleConnsPerHost: mmMaxConns,
			},
		}).
		//SetBaseURL(conf.URL).
		SetBaseURL(mattermostAPIURL).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		//SetHeader("Authorization", "BEARER "+conf.Token).
		OnBeforeRequest(func(_ *resty.Client, r *resty.Request) error {
			token, err := conf.AccessTokenProvider.GetAccessToken()
			if err != nil {
				return trace.Wrap(err)
			}
			fmt.Println("=== BEFORE REQUEST MATTERMOST ACCESS TOKEN ===", token)
			r.SetHeader("Authorization", "Bearer "+token)
			return nil
		})

	// Error response parsing.
	// client.OnBeforeRequest(func(_ *resty.Client, req *resty.Request) error {
	// 	req.SetError(&ErrorResult{})
	// 	return nil
	// })
	client.OnAfterResponse(func(_ *resty.Client, resp *resty.Response) error {
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

		val, ok := cache.Get(cacheKey)
		if !ok {
			return nil
		}

		res, ok := val.(etagCacheEntry)
		if !ok {
			return trace.Errorf("etag cache entry of unknown type %T", val)
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

func (b Bot) CheckHealth(ctx context.Context) error {
	user, err := b.GetMe(ctx)
	fmt.Printf("=== CHECK HEALTH === %#v\n", user)
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

// Broadcast posts request info to Mattermost.
func (b Bot) Broadcast(ctx context.Context, channels []common.Recipient, reqID string, reqData plugindata.AccessRequestData) (common.SentMessages, error) {
	text, err := b.buildPostText(reqID, reqData)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var data common.SentMessages
	var errors []error

	fmt.Println("=== POSTING TO CHANNELS", channels)

	for _, channel := range channels {
		post := Post{
			ChannelID: channel.ID,
			Message:   text,
		}
		resp, err := b.client.NewRequest().
			SetContext(ctx).
			SetBody(post).
			SetResult(&post).
			Post("api/v4/posts")
		if err != nil {
			errors = append(errors, trace.Wrap(err))
			continue
		}
		fmt.Println("=== RESPONSE ===", string(resp.Body()))

		data = append(data, common.MessageData{ChannelID: channel.ID, MessageID: post.ID})
	}

	return data, trace.NewAggregate(errors...)
}

func (b Bot) PostReviewComment(ctx context.Context, channelID, rootID string, review types.AccessReview) error {
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

func (b Bot) UpdateMessages(ctx context.Context, reqID string, reqData plugindata.AccessRequestData, messagingData common.SentMessages, reviews []types.AccessReview) error {
	text, err := b.buildPostText(reqID, reqData)
	if err != nil {
		return trace.Wrap(err)
	}

	var errors []error
	for _, msg := range messagingData {
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

func (b Bot) buildPostText(reqID string, reqData plugindata.AccessRequestData) (string, error) {
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
	case plugindata.Unresolved:
		status = "PENDING"
		statusEmoji = "⏳"
	case plugindata.ResolvedApproved:
		statusEmoji = "✅"
	case plugindata.ResolvedDenied:
		statusEmoji = "❌"
	case plugindata.ResolvedExpired:
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
		plugindata.AccessRequestData
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

func (b Bot) PostReviewReply(ctx context.Context, channelID, timestamp string, review types.AccessReview) error {
	return nil
}

func (b Bot) FetchRecipient(ctx context.Context, recipient string) (*common.Recipient, error) {
	return &common.Recipient{
		Name: recipient,
		ID:   recipient,
		Kind: "Channel",
		Data: nil,
	}, nil
}
