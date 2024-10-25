/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific languap governing permissions and
limitations under the License.
*/

package email

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/mailgun/mailgun-go/v4"
	"gopkg.in/mail.v2"

	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

const (
	statusEmitTimeout = 10 * time.Second
)

// Mailer is an interface to mail sender
type Mailer interface {
	Send(ctx context.Context, id, recipient, body, references string) (string, error)
}

// SMTPMailer implements SMTP mailer
type SMTPMailer struct {
	dialer      *mail.Dialer
	sender      string
	clusterName string
	sink        common.StatusSink
}

// MailgunMailer implements mailgun mailer
type MailgunMailer struct {
	mailgun     *mailgun.MailgunImpl
	sender      string
	clusterName string
}

// NewSMTPMailer inits new SMTP mailer
func NewSMTPMailer(c SMTPConfig, sink common.StatusSink, sender, clusterName string) Mailer {
	dialer := mail.NewDialer(c.Host, c.Port, c.Username, c.Password)
	dialer.StartTLSPolicy = c.MailStartTLSPolicy

	return &SMTPMailer{dialer, sender, clusterName, sink}
}

// NewMailgunMailer inits new Mailgun mailer
func NewMailgunMailer(c MailgunConfig, sink common.StatusSink, sender, clusterName string) Mailer {
	m := mailgun.NewMailgun(c.Domain, c.PrivateKey)
	if c.APIBase != "" {
		m.SetAPIBase(c.APIBase)
	}
	client := &http.Client{
		Transport: &statusSinkTransport{
			RoundTripper: http.DefaultTransport,
			sink:         sink,
		},
	}
	m.SetClient(client)
	return &MailgunMailer{m, sender, clusterName}
}

// Send sends email via SMTP
func (m *SMTPMailer) Send(ctx context.Context, id, recipient, body, references string) (string, error) {
	log := logger.Get(ctx)
	subject := fmt.Sprintf("%v Role Request %v", m.clusterName, id)
	refHeader := fmt.Sprintf("<%v>", references)

	id, err := m.genMessageID()
	if err != nil {
		return "", trace.Wrap(err)
	}

	msg := mail.NewMessage()

	msg.SetHeader("From", m.sender)
	msg.SetHeader("To", recipient)
	msg.SetHeader("Subject", subject)
	msg.SetHeader("Message-ID", fmt.Sprintf("<%v>", id))
	msg.SetBody("text/plain", body)

	if references != "" {
		msg.SetHeader("References", refHeader)
		msg.SetHeader("In-Reply-To", refHeader)
	}

	err = m.dialer.DialAndSend(msg)
	if err != nil {
		if m.sink != nil {
			// Returned error is undocumented. Using geneirc error code for all errors.
			status := common.StatusFromStatusCode(http.StatusInternalServerError)
			if err := m.sink.Emit(ctx, status); err != nil {
				log.WithError(err).Errorf("Error while emitting Email plugin status: %v", err)
			}
		}
		return "", trace.Wrap(err)
	}
	return id, nil
}

// genMessageID generates Message-ID header value
func (m *SMTPMailer) genMessageID() (string, error) {
	now := uint64(time.Now().UnixNano())

	nonceByte := make([]byte, 8)
	if _, err := rand.Read(nonceByte); err != nil {
		return "", trace.Wrap(err)
	}
	nonce := binary.BigEndian.Uint64(nonceByte)

	hostname, err := os.Hostname()
	if err != nil {
		return "", trace.Wrap(err)
	}

	msgID := fmt.Sprintf("%s.%s@%s", m.base36(now), m.base36(nonce), hostname)

	return msgID, nil
}

// base36 converts given value to a base 36 numbering system
func (m *SMTPMailer) base36(input uint64) string {
	return strings.ToUpper(strconv.FormatUint(input, 36))
}

// Send sends email via Mailgun
func (m *MailgunMailer) Send(ctx context.Context, id, recipient, body, references string) (string, error) {
	subject := fmt.Sprintf("%v Role Request %v", m.clusterName, id)
	refHeader := fmt.Sprintf("<%v>", references)

	msg := m.mailgun.NewMessage(m.sender, subject, body, recipient)
	msg.SetRequireTLS(true)

	if references != "" {
		msg.AddHeader("References", refHeader)
		msg.AddHeader("In-Reply-To", refHeader)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	_, id, err := m.mailgun.Send(ctx, msg)

	if err != nil {
		return "", trace.Wrap(err)
	}

	return id, nil
}

// statusSinkTransport wraps the Mailgun client transport and
// emits plugin status.
type statusSinkTransport struct {
	http.RoundTripper
	sink common.StatusSink
}

// RoundTrip implements the http.RoundTripper interface.
func (t *statusSinkTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	log := logger.Get(req.Context())
	resp, err := t.RoundTripper.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if t.sink != nil {
		// No usable context in scope, use background with a reasonable timeout
		ctx, cancel := context.WithTimeout(context.Background(), statusEmitTimeout)
		defer cancel()

		status := common.StatusFromStatusCode(resp.StatusCode)
		if err := t.sink.Emit(ctx, status); err != nil {
			log.WithError(err).Errorf("Error while emitting Email plugin status: %v", err)
		}
	}
	return resp, nil
}
