/*
Copyright 2015-2021 Gravitational, Inc.

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

package email

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

var reviewReplyTemplate = template.Must(template.New("review reply").Parse(
	`{{.Author}} reviewed the request at {{.Created.Format .TimeFormat}}.
Resolution: {{.ProposedStateEmoji}} {{.ProposedState}}.
{{if .Reason}}Reason: {{.Reason}}.{{end}}`,
))

// Client is a email client that works with access.Request
type Client struct {
	clusterName string
	mailer      Mailer
	webProxyURL *url.URL
}

// NewClient initializes the new Email message client
func NewClient(ctx context.Context, conf Config, clusterName, webProxyAddr string) (Client, error) {
	var (
		webProxyURL *url.URL
		err         error
		mailer      Mailer
	)

	if webProxyAddr != "" {
		if webProxyURL, err = lib.AddrToURL(webProxyAddr); err != nil {
			return Client{}, trace.Wrap(err)
		}
	}

	if conf.Mailgun != nil {
		mailer = NewMailgunMailer(*conf.Mailgun, conf.StatusSink, conf.Delivery.Sender, clusterName, conf.RoleToRecipients[types.Wildcard])
		logger.Get(ctx).InfoContext(ctx, "Using Mailgun as email transport", "domain", conf.Mailgun.Domain)
	}

	if conf.SMTP != nil {
		mailer = NewSMTPMailer(*conf.SMTP, conf.StatusSink, conf.Delivery.Sender, clusterName)
		logger.Get(ctx).InfoContext(ctx, "Using SMTP as email transport",
			"host", conf.SMTP.Host,
			"port", conf.SMTP.Port,
			"username", conf.SMTP.Username,
		)
	}

	return Client{
		mailer:      mailer,
		clusterName: clusterName,
		webProxyURL: webProxyURL,
	}, nil
}

// CheckHealth checks if the Email client connection is healthy.
func (c *Client) CheckHealth(ctx context.Context) error {
	return trace.Wrap(c.mailer.CheckHealth(ctx))
}

// SendNewThreads sends emails on new requests. Returns EmailData.
func (c *Client) SendNewThreads(ctx context.Context, recipients []common.Recipient, reqID string, reqData RequestData) ([]EmailThread, error) {
	var threads []EmailThread
	var errors []error

	body := c.buildBody(reqID, reqData, "You have a new Role Request")

	for _, recipient := range recipients {
		id, err := c.mailer.Send(ctx, reqID, recipient.ID, body, "")
		if err != nil {
			errors = append(errors, err)
			continue
		}

		threads = append(threads, EmailThread{Email: recipient.ID, Timestamp: time.Now().String(), MessageID: id})
	}

	return threads, trace.NewAggregate(errors...)
}

// SendReview sends new AccessReview message to the given threads
func (c *Client) SendReview(ctx context.Context, threads []EmailThread, reqID string, reqData RequestData, review types.AccessReview) ([]EmailThread, error) {
	var proposedStateEmoji string
	var threadsSent = make([]EmailThread, 0)

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
		return threadsSent, trace.Wrap(err)
	}
	body := builder.String()

	errors := make([]error, 0)

	for _, thread := range threads {
		_, err = c.mailer.Send(ctx, reqID, thread.Email, body, thread.MessageID)
		if err != nil {
			errors = append(errors, trace.Wrap(err))
			continue
		}
		threadsSent = append(threadsSent, thread)
	}

	return threadsSent, trace.NewAggregate(errors...)
}

// SendResolution sends message on a request status update (review, decline)
func (c *Client) SendResolution(ctx context.Context, threads []EmailThread, reqID string, reqData RequestData) ([]EmailThread, error) {
	var errors []error
	var threadsSent = make([]EmailThread, 0)

	body := c.buildBody(reqID, reqData, "Role Request has been resolved")

	for _, thread := range threads {
		_, err := c.mailer.Send(ctx, reqID, thread.Email, body, thread.MessageID)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		threadsSent = append(threads, thread)
	}

	return threadsSent, trace.NewAggregate(errors...)
}

// buildBody builds a email message for create/resolve events
func (c *Client) buildBody(reqID string, reqData RequestData, subject string) string {
	var builder strings.Builder
	builder.Grow(128)

	builder.WriteString(fmt.Sprintf("%v:\n\n", subject))

	resolution := reqData.Resolution

	msgFieldToBuilder(&builder, "ID", reqID)
	msgFieldToBuilder(&builder, "Cluster", c.clusterName)

	if len(reqData.User) > 0 {
		msgFieldToBuilder(&builder, "User", reqData.User)
	}
	if reqData.Roles != nil {
		msgFieldToBuilder(&builder, "Role(s)", strings.Join(reqData.Roles, ","))
	}
	if reqData.RequestReason != "" {
		msgFieldToBuilder(&builder, "Reason", reqData.RequestReason)
	}
	if c.webProxyURL != nil {
		reqURL := *c.webProxyURL
		reqURL.Path = lib.BuildURLPath("web", "requests", reqID)
		msgFieldToBuilder(&builder, "Link", reqURL.String())
	} else {
		if resolution.Tag == Unresolved {
			msgFieldToBuilder(&builder, "Approve", fmt.Sprintf("tsh request review --approve %s", reqID))
			msgFieldToBuilder(&builder, "Deny", fmt.Sprintf("tsh request review --deny %s", reqID))
		}
	}

	var statusEmoji string
	status := string(resolution.Tag)
	switch resolution.Tag {
	case Unresolved:
		status = "PENDING"
		statusEmoji = "⏳"
	case ResolvedApproved:
		statusEmoji = "✅"
	case ResolvedDenied:
		statusEmoji = "❌"
	case ResolvedExpired:
		statusEmoji = "⌛"
	}

	statusText := fmt.Sprintf("Status: %s %s", statusEmoji, status)
	if resolution.Reason != "" {
		statusText += fmt.Sprintf(" (%s)", resolution.Reason)
	}

	builder.WriteString("\n")
	builder.WriteString(statusText)

	return builder.String()
}

// msgFieldToBuilder utility string builder method
func msgFieldToBuilder(b *strings.Builder, field, value string) {
	b.WriteString(field)
	b.WriteString(": ")
	b.WriteString(value)
	b.WriteString("\n")
}
