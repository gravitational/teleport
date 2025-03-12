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
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
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
	// log is the logger
	log *slog.Logger
	// StatusSink receives any status updates from the plugin for
	// further processing. Status updates will be ignored if not set.
	StatusSink common.StatusSink
}

// NewBot creates new bot struct
func NewBot(c *Config, clusterName, webProxyAddr string, log *slog.Logger) (*Bot, error) {
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
		Config:      c.MSAPI,
		graphClient: msapi.NewGraphClient(c.MSAPI),
		botClient:   msapi.NewBotFrameworkClient(c.MSAPI),
		recipients:  make(map[string]RecipientData),
		webProxyURL: webProxyURL,
		clusterName: clusterName,
		mu:          &sync.RWMutex{},
		log:         log,
		StatusSink:  c.StatusSink,
	}

	return bot, nil
}

// GetTeamsApp finds the application in org store and caches it in a bot instance
func (b *Bot) GetTeamsApp(ctx context.Context) (*msapi.TeamsApp, error) {
	b.log.DebugContext(ctx, "Retrieving the Teams application from the organization store", "teams_app_id", b.Config.TeamsAppID)
	teamsApp, err := b.graphClient.GetTeamsApp(ctx, b.Config.TeamsAppID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	b.log.DebugContext(ctx, "Retrieved the Teams application from the organization store",
		slog.Group("teams_app",
			"id", teamsApp.ID,
			"display_name", teamsApp.DisplayName,
			"external_id", teamsApp.ExternalID,
			"distribution_method", teamsApp.DistributionMethod),
	)
	b.teamsApp = teamsApp
	return b.teamsApp, nil
}

// GetUserIDByEmail gets a user ID by email. NotFoundError if not found.
func (b *Bot) GetUserIDByEmail(ctx context.Context, email string) (string, error) {
	b.log.DebugContext(ctx, "Resolving user ID via email", "email", email)
	user, err := b.graphClient.GetUserByEmail(ctx, email)
	if trace.IsNotFound(err) {
		b.log.DebugContext(ctx, "Failed to resolve user ID via email", "email", email, "error", err)
		return "", trace.Wrap(err, "try user id instead")
	} else if err != nil {
		b.log.DebugContext(ctx, "Failed to resolve user ID via email", "email", email, "error", err)
		return "", trace.Wrap(err)
	}
	b.log.DebugContext(ctx, "Resolved user ID via email", "email", email, "user_id", user.ID)

	return user.ID, nil
}

func (b *Bot) UninstallAppForUser(ctx context.Context, userIDOrEmail string) error {
	if b.teamsApp == nil {
		return trace.Errorf("Bot is not configured, run GetTeamsApp first")
	}
	b.log.DebugContext(ctx, "Starting to uninstall app for user", "user_id_or_email", userIDOrEmail, "teams_app_id", b.teamsApp.ID)

	userID, err := b.getUserID(ctx, userIDOrEmail)
	if err != nil {
		return trace.Wrap(err)
	}
	log := b.log.With("user_id", userID, "user_id_or_email", userIDOrEmail, "teams_app_id", b.Config.TeamsAppID)

	log.DebugContext(ctx, "Retrieving the app installation ID for the user")
	installedApp, err := b.graphClient.GetAppForUser(ctx, b.teamsApp, userID)
	if trace.IsNotFound(err) {
		// App is already uninstalled, nothing to do
		log.DebugContext(ctx, "App is already not installed for user")
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	log.DebugContext(ctx, "Removing the app from the user's installed apps")
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

	log := b.log.With("recipient", recipient)
	log.DebugContext(ctx, "Fetching recipient")

	b.mu.RLock()
	d, ok := b.recipients[recipient]
	b.mu.RUnlock()
	if ok {
		log.DebugContext(ctx, "Found recipient in cache",
			slog.Group("recipient",
				"id", d.ID,
				"installed_app_id", d.App.ID,
				"chat_id", d.Chat.ID,
				"chat_url", d.Chat.WebURL,
				"chat_tenant_id", d.Chat.TenantID,
				"kind", d.Kind,
			),
		)
		return &d, nil
	}

	log.DebugContext(ctx, "Recipient not in cache")
	// Check if the recipient is a channel
	channel, isChannel := b.checkChannelURL(recipient)
	if isChannel {
		log.DebugContext(ctx, "Recipient is a valid channel",
			slog.Group("channel",
				"name", channel.Name,
				"chat_id", channel.ChatID,
				"group", channel.Group,
				"tenant_id", channel.Tenant,
				"url", channel.URL,
			),
		)
		// A team and a group are different but in MsTeams the team is associated to a group and will have the same id.

		log = log.With("teams_app_id", b.teamsApp.ID, "channel_group", channel.Group)
		log.DebugContext(ctx, "Retrieving the app installation ID for the team")
		installedApp, err := b.graphClient.GetAppForTeam(ctx, b.teamsApp, channel.Group)
		if err != nil {
			log.ErrorContext(ctx, "Failed to retrieve the app installation ID for the team", "error", err)
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
		log.DebugContext(ctx, "Retrieved the app installation ID for the team", "recipient_installed_app_id", installedApp.ID)

		// If the recipient is not a channel, it means it is a user (either email or userID)
	} else {
		log.DebugContext(ctx, "Recipient is a user")
		userID, err := b.getUserID(ctx, recipient)
		if err != nil {
			log.ErrorContext(ctx, "Failed to resolve recipient", "error", err)
			return &RecipientData{}, trace.Wrap(err)
		}
		log = log.With("user_id", userID)
		log.DebugContext(ctx, "Successfully resolve user recipient")

		var installedApp *msapi.InstalledApp

		log = log.With("teams_app_id", b.teamsApp.ID)
		log.DebugContext(ctx, "Checking if app is already installed for user")
		installedApp, err = b.graphClient.GetAppForUser(ctx, b.teamsApp, userID)
		if trace.IsNotFound(err) {
			log.DebugContext(ctx, "App is not installed for user, attempting to install it")
			err := b.graphClient.InstallAppForUser(ctx, userID, b.teamsApp.ID)
			// If two installations are running at the same time, one of them will return "Conflict".
			// This status code is OK to ignore as it means the app is already installed.
			if err != nil && msapi.GetErrorCode(err) != http.StatusText(http.StatusConflict) {
				log.ErrorContext(ctx, "Failed to install app for user", "error", err)
				return nil, trace.Wrap(err)
			}

			log.DebugContext(ctx, "App installed for the user, retrieving the installation ID")
			installedApp, err = b.graphClient.GetAppForUser(ctx, b.teamsApp, userID)
			if err != nil {
				log.ErrorContext(ctx, "Cannot retrieve app installation ID for user")
				return nil, trace.Wrap(err, "Failed to install app %v for user %v", b.teamsApp.ID, userID)
			}
		} else if err != nil {
			return nil, trace.Wrap(err)
		}
		log = log.With("recipient_installed_app_id", installedApp.ID)
		log.DebugContext(ctx, "Successfully resolved app installation ID for user")

		log.DebugContext(ctx, "Looking up the installed app chat ID")
		chat, err := b.graphClient.GetChatForInstalledApp(ctx, userID, installedApp.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		log.DebugContext(ctx, "Found the chat ID for the user", "chat_id", chat.ID)

		d = RecipientData{userID, *installedApp, chat, RecipientKindUser}
	}

	b.mu.Lock()
	b.recipients[recipient] = d
	b.mu.Unlock()

	return &d, nil
}

// getUserID takes a userID or an email, checks if it exists, and returns the userID.
func (b *Bot) getUserID(ctx context.Context, userIDOrEmail string) (string, error) {
	b.log.DebugContext(ctx, "Resolving user", "user_id_or_email", userIDOrEmail)
	if lib.IsEmail(userIDOrEmail) {
		b.log.DebugContext(ctx, "User looks like an email", "user_id_or_email", userIDOrEmail)
		uid, err := b.GetUserIDByEmail(ctx, userIDOrEmail)
		if err != nil {
			return "", trace.Wrap(err)
		}

		return uid, nil
	}
	b.log.DebugContext(ctx, "User does not look like an email, resolving via user ID", "user_id_or_email", userIDOrEmail)
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

	log := b.log.With("recipient", recipient, "recipient_installed_app_id", recipientData.App.ID, "recipient_chat_id", recipientData.Chat.ID)
	if updateID == "" {
		log.DebugContext(ctx, "Posting a message")
	} else {
		log.DebugContext(ctx, "Updating a message", "update_id", updateID)
	}

	id, err := b.botClient.PostAdaptiveCardActivity(
		ctx, recipientData.App.ID, recipientData.Chat.ID, cardBody, updateID,
	)
	if err != nil {
		log.ErrorContext(ctx, "Failed to send message")
		return "", trace.Wrap(err)
	}

	log.DebugContext(ctx, "Message sent", "message_id", id)
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

	b.log.DebugContext(ctx, "Looping through every recipient", "id", id, "recipients", recipients)
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

	b.log.DebugContext(ctx, "Updating messages", "id", id, "message_count", len(data.TeamsData))
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
func (b *Bot) checkChannelURL(recipient string) (*Channel, bool) {
	// This context is solely for logging purposes
	ctx := context.Background()
	log := b.log.With("recipient", recipient)

	log.DebugContext(ctx, "Checking if the recipient is a channel")
	channelURL, err := url.Parse(recipient)
	if err != nil {
		log.DebugContext(ctx, "Cannot parse recipient as a URL", "error", err)
		return nil, false
	}

	log.DebugContext(ctx, "Recipient is a valid URL")
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
		log.DebugContext(ctx, "The URL query is missing tenantID or groupID, this is not a channel")
		return nil, false
	}

	// There is no risk to have a channelName with a "/" as they are url-encoded twice
	path := strings.Split(channelURL.Path, "/")
	if len(path) != 5 {
		log.DebugContext(ctx, "The URL path length is not 5, this is not a channel")
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
	log.DebugContext(ctx, "The recipient is a channel",
		slog.Group("channel",
			"name", channel.Name,
			"group_id", channel.Group,
			"tenant_id", channel.Tenant,
			"chat_id", channel.ChatID,
		),
	)

	return &channel, true
}

// CheckHealth checks if the bot can connect to its messaging service
func (b *Bot) CheckHealth(ctx context.Context) error {
	_, err := b.graphClient.GetTeamsApp(ctx, b.Config.TeamsAppID)
	if b.StatusSink != nil {
		status := types.PluginStatusCode_RUNNING
		message := ""
		if err != nil {
			status = types.PluginStatusCode_OTHER_ERROR
			message = err.Error()
		}
		if err := b.StatusSink.Emit(ctx, &types.PluginStatusV1{
			Code:         status,
			ErrorMessage: message,
		}); err != nil {
			b.log.ErrorContext(ctx, "Error while emitting ms teams plugin status", "error", err)
		}
	}
	return trace.Wrap(err)
}
