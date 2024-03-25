// Copyright 2024 Gravitational, Inc
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

package msteams

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/msteams/msapi"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/plugindata"
)

const (
	RecipientKindUser    RecipientKind = "user"
	RecipientKindChannel RecipientKind = "channel"
)

type RecipientKind string

// RecipientData represents cached data for a recipient (user or channel)
type RecipientData struct {
	// ID identifies the recipient, for users it is the UserID, for channels it is "tenant/group/channelName"
	ID string
	// App installation for the recipient
	App msapi.InstalledApp
	// Chat for the recipient
	Chat msapi.Chat
	// Kind of the recipient (user or channel)
	Kind RecipientKind
}

// Channel represents a MSTeams channel parsed from its web URL
type Channel struct {
	Name   string
	Group  string
	Tenant string
	URL    url.URL
	ChatID string
}

// Bot represents the facade to MS Teams API
type Bot struct {
	// Config MS API configuration
	msapi.Config
	// teamsApp represents MS Teams app installed for an org
	teamsApp *msapi.TeamsApp
	// graphClient represents MS API Graph client
	graphClient *msapi.GraphClient
	// botClient represents MS Bot Framework client
	botClient *msapi.BotFrameworkClient
	// mu recipients access mutex
	mu *sync.RWMutex
	// recipients represents the cache of potential message recipients
	recipients map[string]RecipientData
	// webProxyURL represents Web UI address, if enabled
	webProxyURL *url.URL
	// clusterName cluster name
	clusterName string
}

// NewBot creates new bot struct
func NewBot(c msapi.Config, clusterName, webProxyAddr string) (*Bot, error) {
	var (
		webProxyURL *url.URL
		err         error
	)

	if webProxyAddr != "" {
		webProxyURL, err = lib.AddrToURL(webProxyAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	bot := &Bot{
		Config:      c,
		graphClient: msapi.NewGraphClient(c),
		botClient:   msapi.NewBotFrameworkClient(c),
		recipients:  make(map[string]RecipientData),
		webProxyURL: webProxyURL,
		clusterName: clusterName,
		mu:          &sync.RWMutex{},
	}

	return bot, nil
}

// GetTeamsApp finds the application in org store and caches it in a bot instance
func (b *Bot) GetTeamsApp(ctx context.Context) (*msapi.TeamsApp, error) {
	teamsApp, err := b.graphClient.GetTeamsApp(ctx, b.Config.TeamsAppID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	b.teamsApp = teamsApp
	return b.teamsApp, nil
}

// GetUserIDByEmail gets a user ID by email. NotFoundError if not found.
func (b *Bot) GetUserIDByEmail(ctx context.Context, email string) (string, error) {
	user, err := b.graphClient.GetUserByEmail(ctx, email)
	if trace.IsNotFound(err) {
		return "", trace.Wrap(err, "try user id instead")
	} else if err != nil {
		return "", trace.Wrap(err)
	}

	return user.ID, nil
}

// UserExists return true if a user exists. Returns NotFoundError if not found.
func (b *Bot) UserExists(ctx context.Context, id string) error {
	_, err := b.graphClient.GetUserByID(ctx, id)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (b *Bot) UninstallAppForUser(ctx context.Context, userIDOrEmail string) error {
	if b.teamsApp == nil {
		return trace.Errorf("Bot is not configured, run GetTeamsApp first")
	}

	userID, err := b.getUserID(ctx, userIDOrEmail)
	if err != nil {
		return trace.Wrap(err)
	}

	installedApp, err := b.graphClient.GetAppForUser(ctx, b.teamsApp, userID)
	if trace.IsNotFound(err) {
		// App is already uninstalled, nothing to do
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	err = b.graphClient.UninstallAppForUser(ctx, userID, installedApp.ID)
	return trace.Wrap(err)
}

// FetchRecipient checks if recipient is a user or a channel, installs app for a user if missing, fetches chat id
// and saves everything to cache. This method is used for priming the cache. Returns trace.NotFound if a
// user was not found.
func (b *Bot) FetchRecipient(ctx context.Context, recipient string) (*RecipientData, error) {
	if b.teamsApp == nil {
		return nil, trace.Errorf("Bot is not configured, run GetTeamsApp first")
	}

	b.mu.RLock()
	d, ok := b.recipients[recipient]
	b.mu.RUnlock()
	if ok {
		return &d, nil
	}

	// Check if the recipient is a channel
	channel, isChannel := checkChannelURL(recipient)
	if isChannel {
		// A team and a group are different but in MsTeams the team is associated to a group and will have the same id.
		installedApp, err := b.graphClient.GetAppForTeam(ctx, b.teamsApp, channel.Group)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		d = RecipientData{
			ID:  fmt.Sprintf("%s/%s/%s", channel.Tenant, channel.Group, channel.Name),
			App: *installedApp,
			Chat: msapi.Chat{
				ID:       channel.ChatID,
				TenantID: channel.Tenant,
				WebURL:   channel.URL.String(),
			},
			Kind: RecipientKindChannel,
		}
		// If the recipient is not a channel, it means it is a user (either email or userID)
	} else {
		userID, err := b.getUserID(ctx, recipient)
		if err != nil {
			return &RecipientData{}, trace.Wrap(err)
		}

		var installedApp *msapi.InstalledApp

		installedApp, err = b.graphClient.GetAppForUser(ctx, b.teamsApp, userID)
		if trace.IsNotFound(err) {
			err := b.graphClient.InstallAppForUser(ctx, userID, b.teamsApp.ID)
			// If two installations are running at the same time, one of them will return "Conflict".
			// This status code is OK to ignore as it means the app is already installed.
			if err != nil && msapi.GetErrorCode(err) != http.StatusText(http.StatusConflict) {
				return nil, trace.Wrap(err)
			}

			installedApp, err = b.graphClient.GetAppForUser(ctx, b.teamsApp, userID)
			if err != nil {
				return nil, trace.Wrap(err, "Failed to install app %v for user %v", b.teamsApp.ID, userID)
			}
		} else if err != nil {
			return nil, trace.Wrap(err)
		}

		chat, err := b.graphClient.GetChatForInstalledApp(ctx, userID, installedApp.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		d = RecipientData{userID, *installedApp, chat, RecipientKindUser}
	}

	b.mu.Lock()
	b.recipients[recipient] = d
	b.mu.Unlock()

	return &d, nil
}

// getUserID takes a userID or an email, checks if it exists, and returns the userID.
func (b *Bot) getUserID(ctx context.Context, userIDOrEmail string) (string, error) {
	if lib.IsEmail(userIDOrEmail) {
		uid, err := b.GetUserIDByEmail(ctx, userIDOrEmail)
		if err != nil {
			return "", trace.Wrap(err)
		}

		return uid, nil
	}
	_, err := b.graphClient.GetUserByID(ctx, userIDOrEmail)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return userIDOrEmail, nil
}

// PostAdaptiveCardActivity sends the AdaptiveCard to a user
func (b *Bot) PostAdaptiveCardActivity(ctx context.Context, recipient, cardBody, updateID string) (string, error) {
	recipientData, err := b.FetchRecipient(ctx, recipient)
	if err != nil {
		return "", trace.Wrap(err)
	}

	id, err := b.botClient.PostAdaptiveCardActivity(
		ctx, recipientData.App.ID, recipientData.Chat.ID, cardBody, updateID,
	)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return id, nil
}

// PostMessages sends a message to a set of recipients. Returns array of TeamsMessage to cache.
func (b *Bot) PostMessages(ctx context.Context, recipients []string, id string, reqData plugindata.AccessRequestData) ([]TeamsMessage, error) {
	var data []TeamsMessage
	var errors []error

	body, err := BuildCard(id, b.webProxyURL, b.clusterName, reqData, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, recipient := range recipients {
		id, err := b.PostAdaptiveCardActivity(ctx, recipient, body, "")
		if err != nil {
			errors = append(errors, trace.Wrap(err))
			continue
		}
		msg := TeamsMessage{
			ID:          id,
			Timestamp:   time.Now().Format(time.RFC822),
			RecipientID: recipient,
		}
		data = append(data, msg)
	}

	if len(errors) == 0 {
		return data, nil
	}

	return data, trace.NewAggregate(errors...)
}

// UpdateMessages posts message updates
func (b *Bot) UpdateMessages(ctx context.Context, id string, data PluginData, reviews []types.AccessReview) error {
	var errors []error

	body, err := BuildCard(id, b.webProxyURL, b.clusterName, data.AccessRequestData, reviews)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, msg := range data.TeamsData {
		_, err := b.PostAdaptiveCardActivity(ctx, msg.RecipientID, body, msg.ID)
		if err != nil {
			errors = append(errors, trace.Wrap(err))
		}
	}

	if len(errors) == 0 {
		return nil
	}

	return trace.NewAggregate(errors...)
}

// checkChannelURL receives a recipient and checks if it is a channel URL.
// If it is the case, the URL is parsed and the channel RecipientData is returned
func checkChannelURL(recipient string) (*Channel, bool) {
	channelURL, err := url.Parse(recipient)
	if err != nil {
		return nil, false
	}

	var tenantID, groupID, channelName, chatID string
	for k, v := range channelURL.Query() {
		switch k {
		case "tenantId":
			tenantID = v[0]
		case "groupId":
			groupID = v[0]
		default:
		}
	}
	if tenantID == "" || groupID == "" {
		return nil, false
	}

	// There is no risk to have a channelName with a "/" as they are url-encoded twice
	path := strings.Split(channelURL.Path, "/")
	if len(path) != 5 {
		return nil, false
	}
	channelName = path[len(path)-1]
	chatID = path[len(path)-2]

	channel := Channel{
		Name:   channelName,
		Group:  groupID,
		Tenant: tenantID,
		URL:    *channelURL,
		ChatID: chatID,
	}

	return &channel, true
}
