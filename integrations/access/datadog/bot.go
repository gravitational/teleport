/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package datadog

import (
	"context"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
)

type Bot struct {
	datadog     *Datadog
	clusterName string
}

// SupportedApps are the apps supported by this bot.
func (b Bot) SupportedApps() []common.App {
	return []common.App{
		accessrequest.NewApp(b),
	}
}

func (b Bot) CheckHealth(ctx context.Context) error {
	return trace.Wrap(b.datadog.CheckHealth(ctx))
}

func (b Bot) SendReviewReminders(ctx context.Context, recipient common.Recipient, accessLists []*accesslist.AccessList) error {
	return trace.NotImplemented("access list review reminder is not implemented for plugin")
}

func (b Bot) BroadcastAccessRequestMessage(ctx context.Context, recipients []common.Recipient, reqID string, reqData pd.AccessRequestData) (accessrequest.SentMessages, error) {
	var data accessrequest.SentMessages
	incidentData, err := b.datadog.CreateIncident(ctx, b.clusterName, reqID, recipients, reqData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data = append(data, accessrequest.MessageData{ChannelID: incidentData.ID, MessageID: incidentData.ID})

	return data, nil
}

func (b Bot) PostReviewReply(ctx context.Context, channelID, _ string, review types.AccessReview) error {
	return trace.Wrap(b.datadog.PostReviewNote(ctx, channelID, review))
}

func (b Bot) NotifyUser(ctx context.Context, reqID string, reqData pd.AccessRequestData) error {
	return trace.NotImplemented("notify user is not implemented for plugin")
}

func (b Bot) UpdateMessages(ctx context.Context, reqID string, reqData pd.AccessRequestData, incidents accessrequest.SentMessages, reviews []types.AccessReview) error {
	var errors []error
	for _, incident := range incidents {
		err := b.datadog.ResolveIncident(ctx, incident.ChannelID, reqData, reviews)
		errors = append(errors, trace.Wrap(err))
	}
	return trace.NewAggregate(errors...)
}

func (b Bot) FetchRecipient(ctx context.Context, name string) (*common.Recipient, error) {
	return &common.Recipient{
		Name: name,
		ID:   name,
		Kind: common.RecipientKindTeam,
		Data: nil,
	}, nil
}

func (b Bot) FetchOncallUsers(ctx context.Context, reqData pd.AccessRequestData) ([]string, error) {
	oncallTeams, err := b.datadog.GetOncallTeams(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	teamNames := getTeamNamesFromAnnotations(reqData, "datadog_auto_approvals")

	log := logger.Get(ctx)
	log.WithField("oncall_teams", oncallTeams).Info("Fetch oncall teams")

	var oncallUserIDs []string
	for _, oncallTeam := range oncallTeams.Data {
		if !slices.Contains(teamNames, strings.TrimSpace(oncallTeam.Attributes.Handle)) && !slices.Contains(teamNames, strings.TrimSpace(oncallTeam.Attributes.Name)) {
			log.WithField("oncall_team_name", oncallTeam.Attributes.Name).
				WithField("oncall_team_handle", oncallTeam.Attributes.Handle).
				Info("oncall team does not match")
			continue
		}
		for _, oncallUser := range oncallTeam.Relationships.OncallUsers.Data {
			oncallUserIDs = append(oncallUserIDs, oncallUser.ID)
		}
	}

	var oncallUserEmails []string
	for _, user := range oncallTeams.Included {
		if slices.Contains(oncallUserIDs, user.ID) {
			oncallUserEmails = append(oncallUserEmails, user.Attributes.Email)
		}
	}

	log.WithField("team_names", teamNames).
		WithField("oncall_user_ids", oncallUserIDs).
		WithField("oncall_user_emails", oncallUserEmails).
		Info("Fetch oncall users")

	return oncallUserEmails, nil
}

func getTeamNamesFromAnnotations(reqData pd.AccessRequestData, annotationKey string) []string {
	teamNames := reqData.SystemAnnotations[annotationKey]
	slices.Sort(teamNames)
	return slices.Compact(teamNames)
}
