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
