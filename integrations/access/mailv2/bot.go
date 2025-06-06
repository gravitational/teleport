package mailv2

import (
	"context"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	pd "github.com/gravitational/teleport/integrations/lib/plugindata"
	"github.com/gravitational/teleport/lib/accessmonitoring/notification"
	"github.com/gravitational/trace"
	"log/slog"
	"strings"
	"text/template"
	"time"
)

var (
	accessRequestDetails = `
Request ID: {{ .Request.ID }}
Teleport Cluster: {{ .ClusterName }}
Requestor: {{ .Request.User }}
Request Reason: {{ .Request.RequestReason }}
{{- if .Request.Roles }}
Requested Roles: {{ range $index, $role := .Request.Roles }}{{if $index}},{{end}}{{ $role }}{{end}}
{{- end }}
{{- if .Request.Resources }}
Requested Resources: {{ range $index, $resource := .Request.Resources }}{{if $index}},{{end}}{{ $resource }}{{end}}
{{- end }}
{{- if .ClusterURL }}
See the request in Teleport: {{ printf "%s/web/requests/%s" .ClusterURL .Request.ID }}
{{- end }}`
	reviewInstructions = `
{{- if not .Request.ResolutionTag }}

Approve: tsh request review --approve {{ .Request.ID | printf "%q" }}
Deny: tsh review --deny {{ .Request.ID | printf "%q" }}
{{- end }}`
	useAccessRequestInstructions = `

Use the requested access by running: tsh login --request-id={{.Request.ID | printf "%q"}}
`

	approverNewAccessRequestBody       = `You have a new Teleport Access Request.` + accessRequestDetails + reviewInstructions
	approverResolvedAccessRequestBody  = `Access Request {{ .Request.ResolutionTag.String }} {{ .Request.ResolutionTag.Emoji }}` + accessRequestDetails
	requestorNewAccessRequestBody      = `You submitted a new Teleport Access Request.` + accessRequestDetails
	requestorResolvedAccessRequestBody = approverResolvedAccessRequestBody + useAccessRequestInstructions

	approverNewAccessRequestTemplate = template.Must(template.New("reviewer new access request").Parse(
		approverNewAccessRequestBody,
	))
	requestorNewAccessRequestTemplate = template.Must(template.New("requestor new access request").Parse(
		requestorNewAccessRequestBody,
	))
	approverResolvedAccessRequestTemplate = template.Must(template.New("approver resolved access request").Parse(
		approverResolvedAccessRequestBody,
	))
	requestorResolvedAccessRequestTemplate = template.Must(template.New("requestor resolved access request").Parse(
		requestorResolvedAccessRequestBody,
	))
	reviewReplyTemplate = template.Must(template.New("review reply").Parse(
		`{{.Author}} reviewed the request at {{.Created.Format .TimeFormat}}.
Resolution: {{.ProposedStateEmoji}} {{.ProposedState}}.
{{if .Reason}}Reason: {{.Reason}}.{{end}}`))
)

type templateInput struct {
	Request     pd.AccessRequestData
	ClusterName string
	ClusterURL  string
}

func NewBot(ctx context.Context, log *slog.Logger, conf *Config, pong proto.PingResponse) notification.Bot[*EmailThread, *ReviewEmail] {
	var (
		mailer Mailer
	)

	if conf.Mailgun != nil {
		mailer = NewMailgunMailer(*conf.Mailgun, conf.StatusSink, conf.Delivery.Sender, pong.ClusterName, conf.RoleToRecipients[types.Wildcard])
		log.InfoContext(ctx, "Using Mailgun as email transport", "domain", conf.Mailgun.Domain)
	}

	if conf.SMTP != nil {
		mailer = NewSMTPMailer(*conf.SMTP, conf.StatusSink, conf.Delivery.Sender, pong.ClusterName)
		log.InfoContext(ctx, "Using SMTP as email transport",
			"host", conf.SMTP.Host,
			"port", conf.SMTP.Port,
			"username", conf.SMTP.Username,
		)
	}

	return &Bot{
		pong:   pong,
		mailer: mailer,
		log:    log,
	}
}

type Bot struct {
	pong   proto.PingResponse
	mailer Mailer
	log    *slog.Logger
}

func (b Bot) UpdateMessage(ctx context.Context, thread *EmailThread, _ []types.AccessReview, reqData pd.AccessRequestData, canReview bool) error {
	// Emails don't support updating already sent emails
	// We don't want to spam the users on every review (they already receive individual emails for every review)
	// So we will only send messages if the request is resolved
	if reqData.ResolutionTag == pd.Unresolved {
		return nil
	}

	var tpl *template.Template
	if canReview {
		tpl = approverResolvedAccessRequestTemplate
	} else {
		tpl = requestorResolvedAccessRequestTemplate
	}

	var sb strings.Builder
	input := templateInput{
		Request:     reqData,
		ClusterName: b.pong.ClusterName,
		ClusterURL:  "https://" + b.pong.ProxyPublicAddr,
	}
	err := tpl.Execute(&sb, input)
	if err != nil {
		return trace.Wrap(err, "failed to execute template")
	}

	body := sb.String()
	_, err = b.mailer.Send(ctx, thread.RequestID, thread.Email, body, thread.MessageID)
	return trace.Wrap(err, "sending resolution email to %q", thread.Email)

}

func (b Bot) CheckHealth(ctx context.Context) error {
	return trace.Wrap(b.mailer.CheckHealth(ctx))
}

func (b Bot) FetchRecipient(ctx context.Context, rawRecipient string) (*common.Recipient, error) {
	if !lib.IsEmail(rawRecipient) {
		b.log.WarnContext(ctx, "Recipient is not a valid email address, skipping", "recipient", rawRecipient)
		return nil, trace.NotFound("recipient is not a valid email address")
	}
	return &common.Recipient{
		ID:   rawRecipient,
		Name: rawRecipient,
		Kind: common.RecipientKindEmail,
	}, nil
}

func (b Bot) NotifyApprover(ctx context.Context, recipient common.Recipient, reqData pd.AccessRequestData) (data *EmailThread, err error) {
	return b.notify(ctx, recipient, reqData, approverNewAccessRequestTemplate)
}

func (b Bot) NotifyRequestor(ctx context.Context, recipient common.Recipient, reqData pd.AccessRequestData) (data *EmailThread, err error) {
	return b.notify(ctx, recipient, reqData, requestorNewAccessRequestTemplate)
}

func (b Bot) notify(ctx context.Context, recipient common.Recipient, reqData pd.AccessRequestData, tpl *template.Template) (data *EmailThread, err error) {
	var sb strings.Builder
	input := templateInput{
		Request:     reqData,
		ClusterName: b.pong.ClusterName,
		ClusterURL:  "https://" + b.pong.ProxyPublicAddr,
	}
	err = tpl.Execute(&sb, input)
	if err != nil {
		return nil, trace.Wrap(err, "failed to execute template")
	}

	body := sb.String()
	const references = ""
	id, err := b.mailer.Send(ctx, reqData.ID, recipient.ID, body, references)
	if err != nil {
		return nil, trace.Wrap(err, "failed to send email to %s", recipient.ID)
	}
	return &EmailThread{
		Email:     recipient.ID,
		MessageID: id,
		Timestamp: time.Now().String(),
		RequestID: reqData.ID,
	}, nil
}

func (b Bot) PostReview(ctx context.Context, thread *EmailThread, review types.AccessReview) (*ReviewEmail, error) {
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
		return nil, trace.Wrap(err)
	}
	body := builder.String()
	reviewMessageID, err := b.mailer.Send(ctx, thread.RequestID, thread.Email, body, thread.MessageID)
	if err != nil {
		return nil, trace.Wrap(err, "sending review email to %s", thread.Email)
	}

	return &ReviewEmail{reviewMessageID}, nil
}

// EmailThread stores value about particular original message
type EmailThread struct {
	Email     string
	MessageID string
	Timestamp string
	RequestID string
}

func (et *EmailThread) ID() notification.MessageID {
	return notification.MessageID(et.Email + "/" + et.MessageID)
}

type ReviewEmail struct {
	MessageID string
}

func (re *ReviewEmail) ID() notification.ReviewID {
	return notification.ReviewID(re.MessageID)
}
