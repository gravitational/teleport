import cfg from 'teleport/config';

// FormDataField defines the hardcoded form field
// names expected by the backend.
export enum FormDataField {
  Service = 'service',
  Sender = 'sender',
  FallbackRecipient = 'fallbackRecipient',

  // Mailgun configuration fields
  Domain = 'domain',
  PrivateKey = 'privateKey',

  // SMTP configuration fields
  Host = 'host',
  Port = 'port',
  StartTLSPolicy = 'startTLSPolicy',
  Username = 'username',
  Password = 'password',
}

export type SupportedEmailService = 'smtp' | 'mailgun';

export const getSupportedEmailServices = (): SupportedEmailService[] =>
  // SMTP is disabled for Cloud-Hosted Teleport
  cfg.isCloud ? ['mailgun'] : ['mailgun', 'smtp'];

export const supportedEmailServiceLabel = (
  service: SupportedEmailService
): string => {
  switch (service) {
    case 'smtp':
      return 'SMTP';
    case 'mailgun':
      return 'Mailgun';
    default:
      service satisfies never;
  }
};
