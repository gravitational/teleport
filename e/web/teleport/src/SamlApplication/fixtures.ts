import { SamlIdpServiceProvider } from 'teleport/services/samlidp/types';

export const idpMetadata = {
  entityID: 'https://tele.dev/enterprise/saml-idp/metadata',
  ssoURL: 'https://tele.dev/enterprise/saml-idp/sso',
  x509PEM:
    '-----BEGIN CERTIFICATE-----\nTUlJRGVqQ0NBbUtnQXdJQkFnSVFQc0FRbmVJM2crNm4yUUt0VEwzbTlUQU5CZ2tx\nazhTTmpvZ1hmYVdWTGY4MlZXR3NpdUpkT1YwUWVTUm1vcFpMRFNjamVGb1FmVjZL\nbDZXQlh1dVZZY2s5ekRYQUdoZ0ZvRHpXYm5KMU5zOEplOXEza3lKVDNOckcwUXVW\nU0xNOXBjOENBd0VBQWFOQ01FQXdEZ1lEVlIwUEFRSC9CQVFEQWdHbU1BOEdBMVVk\ncHc5cWt6VjJ3dkFHZUE1VzdLQm5nbTJEcmtHWk1qbG1nNzJubUs1ZGNPRVJTMmRQ\nOEd3VGpjbTQ0YlNuY2Zray9JVGRhMmN0YnZ5QTBiQTRGSU1RTG4zYTZpL2hjajFZ\nN1RZWUZhSGRxWjRYbU5ENk40N3k5UHlJYXZQUW9uZDlBc1orVStxZjBsd1BxSXJk\nZGYybmdxUFJ3UmxGMjE1SjdmZ21hemxpRVZVWnB3SlRCL1ZrRm01SHgrS2Frdmp6\nZVhjanFlNW9zTjRGN3hHcDdmdzQrNmMrWnFQaTJnSzhSL05iVnRoWGl6N3BENEQr\nTUVyNkg1Q1JwM1kzSmVyVVV1NVVaOFlvc1NDTHgrMDZxUkUxZndiZGpxY3liRHBM\neHNWWmFRdmtySTR6RTYzeFc1cmlML1p5NGppVGVJYi81OXpzVE5tRQ==\n-----END CERTIFICATE-----\n',
};

export const samlIdpServiceProvider: SamlIdpServiceProvider = {
  kind: 'saml_idp_service_provider',
  metadata: {
    name: 'my_app',
    labels: {},
  },
  spec: {
    acs_url: 'https://example.com/saml/acs',
    attribute_mapping: [
      { name: 'role', name_format: 'uri', value: 'user.spec.role' },
    ],
    entity_descriptor: '<><>',
    entity_id: 'https://example.com/saml/eid',
    preset: 'unspecified',
    relay_state: '',
  },
  version: '',
};
