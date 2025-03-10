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
	"fmt"
	"net/url"
	"slices"
	"strings"
	"text/template"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/integrations/access/accessrequest"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
)

// Bot is a Datadog client that works with AccessRequest.
// It is responsible for creating/updating Datadog incidents when access request
// events occur.
type Bot struct {
	datadog     *Datadog
	clusterName string
	webProxyURL *url.URL
}

var incidentSummaryTemplate = template.Must(template.New("incident summary").Parse(
	`You have a new Access Request:

ID: {{.ID}}
Cluster: {{.ClusterName}}
User: {{.User}}
Role(s): {{range $index, $element := .Roles}}{{if $index}}, {{end}}{{ . }}{{end}}
{{if .RequestLink}}Link: {{.RequestLink}}{{end}} `,
))
var reviewNoteTemplate = template.Must(template.New("review note").Parse(
	`{{.Author}} reviewed the request.
Resolution: {{.ProposedState}}.
{{if .Reason}}Reason: {{.Reason}}.{{end}}`,
))
var resolutionNoteTemplate = template.Must(template.New("resolution note").Parse(
	`Access request is {{.Resolution}}
{{if .ResolveReason}}Reason: {{.ResolveReason}}{{end}}`,
))

// SupportedApps are the apps supported by this bot.
func (b Bot) SupportedApps() []common.App {
	return []common.App{
		accessrequest.NewApp(b),
	}
}

// CheckHealth checks if Datadog connection is healthy.
func (b Bot) CheckHealth(ctx context.Context) error {
	return trace.Wrap(b.datadog.CheckHealth(ctx))
}

// SendReviewReminders will send a review reminder that an access list needs to be reviewed.
func (b Bot) SendReviewReminders(ctx context.Context, recipient common.Recipient, accessLists []*accesslist.AccessList) error {
	return trace.NotImplemented("access list review reminder is not implemented for plugin")
}

// BroadcastAccessRequestMessage creates an incident for the provided recipients.
func (b Bot) BroadcastAccessRequestMessage(ctx context.Context, recipients []common.Recipient, reqID string, reqData pd.AccessRequestData) (accessrequest.SentMessages, error) {
	summary, err := buildIncidentSummary(b.clusterName, reqID, reqData, b.webProxyURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	incidentData, err := b.datadog.CreateIncident(ctx, summary, recipients, reqData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var data accessrequest.SentMessages
	data = append(data, accessrequest.MessageData{ChannelID: incidentData.ID, MessageID: incidentData.ID})
	return data, nil
}

// PostReviewReply posts an incident note.
func (b Bot) PostReviewReply(ctx context.Context, channelID, _ string, review types.AccessReview) error {
	note, err := buildReviewNoteBody(review)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(b.datadog.PostReviewNote(ctx, channelID, note))
}

// NotifyUser will send users a direct notice with the access request status.
func (b Bot) NotifyUser(ctx context.Context, reqID string, reqData pd.AccessRequestData) error {
	return trace.NotImplemented("notify user is not implemented for plugin")
}

// UpdateMessages updates the indicent.
func (b Bot) UpdateMessages(ctx context.Context, reqID string, reqData pd.AccessRequestData, incidents accessrequest.SentMessages, reviews []types.AccessReview) error {
	var errors []error

	switch reqData.ResolutionTag {
	case pd.ResolvedApproved, pd.ResolvedDenied, pd.ResolvedExpired:
	default:
		// If the incident is not resolved, we don't need to post any resolution message
		// Nor to change its state. Un-resolving an access request should be impossible.
		// We can return immediately, nothing to update in the incident.
		return nil
	}

	note, err := buildResolutionNoteBody(reqData)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, incident := range incidents {
		if err := b.datadog.PostReviewNote(ctx, incident.ChannelID, note); err != nil {
			errors = append(errors, trace.Wrap(err))
			continue
		}
		err := b.datadog.ResolveIncident(ctx, incident.ChannelID, "resolved")
		errors = append(errors, trace.Wrap(err))
	}
	return trace.NewAggregate(errors...)
}

// FetchRecipient fetches the  recipient for the given name.
func (b Bot) FetchRecipient(ctx context.Context, name string) (*common.Recipient, error) {
	var kind string
	if lib.IsEmail(name) {
		kind = common.RecipientKindEmail
		name = fmt.Sprintf("@%s", name)
	} else {
		kind = common.RecipientKindTeam
	}
	return &common.Recipient{
		Name: name,
		ID:   name,
		Kind: kind,
	}, nil
}

// FetchOncallUsers fetches on-call users filtered by the provided annotations.
func (b Bot) FetchOncallUsers(ctx context.Context, req types.AccessRequest) ([]string, error) {
	log := logger.Get(ctx)

	annotationKey := types.TeleportNamespace + types.ReqAnnotationApproveSchedulesLabel
	teamNames, err := common.GetNamesFromAnnotations(req, annotationKey)
	if err != nil {
		log.DebugContext(ctx, "Automatic approvals annotation is empty or unspecified")
		return nil, nil
	}

	// Fetch all on-call teams
	oncallTeams, err := b.datadog.GetOncallTeams(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var oncallUserIDs []string
	for _, oncallTeam := range oncallTeams.Data {
		// Filter the list of teams to only the teams that match the datadog annotation.
		if !slices.Contains(teamNames, oncallTeam.Attributes.Handle) &&
			!slices.Contains(teamNames, oncallTeam.Attributes.Name) {
			continue
		}
		// Collect users that are on-call for the specified team.
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
	return oncallUserEmails, nil
}

func buildIncidentSummary(clusterName, reqID string, reqData pd.AccessRequestData, webProxyURL *url.URL) (string, error) {
	var requestLink string
	if webProxyURL != nil {
		reqURL := *webProxyURL
		reqURL.Path = lib.BuildURLPath("web", "requests", reqID)
		requestLink = reqURL.String()
	}

	var builder strings.Builder
	err := incidentSummaryTemplate.Execute(&builder, struct {
		ID          string
		ClusterName string
		RequestLink string
		pd.AccessRequestData
	}{
		reqID,
		clusterName,
		requestLink,
		reqData,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}

func buildReviewNoteBody(review types.AccessReview) (string, error) {
	var builder strings.Builder
	err := reviewNoteTemplate.Execute(&builder, struct {
		Author        string
		ProposedState string
		Reason        string
	}{
		review.Author,
		review.ProposedState.String(),
		review.Reason,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}

func buildResolutionNoteBody(reqData pd.AccessRequestData) (string, error) {
	var builder strings.Builder
	err := resolutionNoteTemplate.Execute(&builder, struct {
		Resolution    string
		ResolveReason string
	}{
		statusText(reqData.ResolutionTag),
		reqData.ResolutionReason,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}

func statusText(tag pd.ResolutionTag) string {
	var statusEmoji string
	status := string(tag)
	switch tag {
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
	return fmt.Sprintf("%s %s", statusEmoji, status)
}
