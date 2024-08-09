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

package accessrequest

import (
	"fmt"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
)

// Slack has a 4000 character limit for message texts and 3000 character limit
// for message section texts, so we truncate all reasons to a generous but
// conservative limit
const (
	requestInlineLimit = 500
	requestReasonLimit
	resolutionReasonLimit
	ReviewReasonLimit
)

var reviewReplyTemplate = template.Must(template.New("review reply").Parse(
	`{{.Author}} reviewed the request at {{.Created.Format .TimeFormat}}.
Resolution: {{.ProposedStateEmoji}} {{.ProposedState}}.
{{if .Reason}}Reason: {{.Reason}}.{{end}}`,
))

func MsgStatusText(tag pd.ResolutionTag, reason string) string {
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

	statusText := fmt.Sprintf("*Status*: %s %s", statusEmoji, status)
	if reason != "" {
		statusText += fmt.Sprintf("\n*Resolution reason*: %s", lib.MarkdownEscape(reason, resolutionReasonLimit))
	}

	return statusText
}

func MsgFields(reqID string, reqData pd.AccessRequestData, clusterName string, webProxyURL *url.URL) string {
	var builder strings.Builder
	builder.Grow(128)

	msgFieldToBuilder(&builder, "ID", reqID)
	msgFieldToBuilder(&builder, "Cluster", clusterName)

	if len(reqData.User) > 0 {
		msgFieldToBuilder(&builder, "User", reqData.User)
	}
	if len(reqData.Roles) > 0 {
		for role, logins := range reqData.LoginsByRole {
			var loginStr string
			if len(logins) > 0 {
				loginStr = strings.Join(logins, ", ")
			} else {
				loginStr = "none"
			}
			msgFieldToBuilder(&builder, "Role", lib.MarkdownEscapeInLine(role, requestInlineLimit),
				"Login(s)", lib.MarkdownEscapeInLine(loginStr, requestInlineLimit))
		}

	}
	if len(reqData.Resources) > 0 {
		msgFieldToBuilder(&builder, "Resource(s)", lib.MarkdownEscapeInLine(strings.Join(reqData.Resources, ","), requestInlineLimit))
	}
	if reqData.RequestReason != "" {
		msgFieldToBuilder(&builder, "Reason", lib.MarkdownEscape(reqData.RequestReason, requestReasonLimit))
	}
	if webProxyURL != nil {
		reqURL := *webProxyURL
		reqURL.Path = lib.BuildURLPath("web", "requests", reqID)
		msgFieldToBuilder(&builder, "Link", reqURL.String())
	} else {
		if reqData.ResolutionTag == pd.Unresolved {
			msgFieldToBuilder(&builder, "Approve", fmt.Sprintf("`tsh request review --approve %s`", reqID))
			msgFieldToBuilder(&builder, "Deny", fmt.Sprintf("`tsh request review --deny %s`", reqID))
		}
	}

	return builder.String()
}

func MsgReview(review types.AccessReview) (string, error) {
	if review.Reason != "" {
		review.Reason = lib.MarkdownEscape(review.Reason, ReviewReasonLimit)
	}

	var proposedStateEmoji string
	switch review.ProposedState {
	case types.RequestState_APPROVED:
		proposedStateEmoji = "✅"
	case types.RequestState_DENIED:
		proposedStateEmoji = "❌"
	}

	var builder strings.Builder
	err := reviewReplyTemplate.Execute(&builder, struct {
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
		return "", trace.Wrap(err)
	}
	return builder.String(), nil
}

func msgFieldToBuilder(b *strings.Builder, field, value string, additionalFields ...string) {
	b.WriteString("*")
	b.WriteString(field)
	b.WriteString("*: ")
	b.WriteString(value)

	for i := 0; i < len(additionalFields)-1; i += 2 {
		field := additionalFields[i]
		value := additionalFields[i+1]
		b.WriteString(" *")
		b.WriteString(field)
		b.WriteString("*: ")
		b.WriteString(value)
	}

	b.WriteString("\n")
}
