package plugins

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/email"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

func emailInstanceFactory(_ context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error) {
	emailSpec := plugin.Spec.GetEmail()
	if emailSpec == nil {
		return nil, trace.BadParameter("field Spec.Email must be present")
	}

	if len(deps.staticCredentials) == 0 {
		return nil, trace.BadParameter("missing Email plugin static credentials")
	}

	cfg := email.Config{
		Delivery: email.DeliveryConfig{
			Sender: emailSpec.Sender,
		},
		RoleToRecipients: common.RawRecipientsMap{
			types.Wildcard: []string{emailSpec.FallbackRecipient},
		},
		StatusSink: deps.statusSink,
		Client:     deps.client,
	}

	switch spec := emailSpec.GetSpec().(type) {
	case *types.PluginEmailSettings_MailgunSpec:
		cfg.Mailgun = &email.MailgunConfig{
			Domain:     emailSpec.GetMailgunSpec().Domain,
			PrivateKey: deps.staticCredentials[0].GetAPIToken(),
		}
	case *types.PluginEmailSettings_SmtpSpec:
		smtpSpec := emailSpec.GetSmtpSpec()
		cfg.SMTP = &email.SMTPConfig{
			Host:           smtpSpec.Host,
			Port:           int(smtpSpec.Port),
			StartTLSPolicy: smtpSpec.StartTlsPolicy,
		}
		cfg.SMTP.Username, cfg.SMTP.Password = deps.staticCredentials[0].GetBasicAuth()
	default:
		return nil, trace.BadParameter("unknown email spec: %T", spec)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	app, err := email.NewApp(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appCtx := logger.WithLogger(deps.lifetime, nil)
	return func() error {
		return trace.Wrap(app.Run(appCtx))
	}, nil
}
