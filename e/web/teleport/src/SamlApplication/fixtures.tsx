import Validation from 'shared/components/Validation';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  emptyUpsertRequest,
  SamlApplicationProvider,
} from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { ContextProvider } from 'teleport';
import { SamlMeta } from 'teleport/Discover/useDiscover';
import {
  SamlIdpServiceProvider,
  SamlServiceProviderPreset,
} from 'teleport/services/samlidp/types';

export const idpMetadata = {
  entityID: 'https://tele.dev/enterprise/saml-idp/metadata',
  ssoURL: 'https://tele.dev/enterprise/saml-idp/sso',
  x509PEM:
    '-----BEGIN CERTIFICATE-----\nTUlJRGVqQ0NBbUtnQXdJQkFnSVFQc0FRbmVJM2crNm4yUUt0VEwzbTlUQU5CZ2tx\nazhTTmpvZ1hmYVdWTGY4MlZXR3NpdUpkT1YwUWVTUm1vcFpMRFNjamVGb1FmVjZL\nbDZXQlh1dVZZY2s5ekRYQUdoZ0ZvRHpXYm5KMU5zOEplOXEza3lKVDNOckcwUXVW\nU0xNOXBjOENBd0VBQWFOQ01FQXdEZ1lEVlIwUEFRSC9CQVFEQWdHbU1BOEdBMVVk\ncHc5cWt6VjJ3dkFHZUE1VzdLQm5nbTJEcmtHWk1qbG1nNzJubUs1ZGNPRVJTMmRQ\nOEd3VGpjbTQ0YlNuY2Zray9JVGRhMmN0YnZ5QTBiQTRGSU1RTG4zYTZpL2hjajFZ\nN1RZWUZhSGRxWjRYbU5ENk40N3k5UHlJYXZQUW9uZDlBc1orVStxZjBsd1BxSXJk\nZGYybmdxUFJ3UmxGMjE1SjdmZ21hemxpRVZVWnB3SlRCL1ZrRm01SHgrS2Frdmp6\nZVhjanFlNW9zTjRGN3hHcDdmdzQrNmMrWnFQaTJnSzhSL05iVnRoWGl6N3BENEQr\nTUVyNkg1Q1JwM1kzSmVyVVV1NVVaOFlvc1NDTHgrMDZxUkUxZndiZGpxY3liRHBM\neHNWWmFRdmtySTR6RTYzeFc1cmlML1p5NGppVGVJYi81OXpzVE5tRQ==\n-----END CERTIFICATE-----\n',
};

export const mockSamlIdpServiceProvider: SamlIdpServiceProvider = {
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
    preset: SamlServiceProviderPreset.Unspecified,
    relay_state: '',
  },
  version: '',
};

export const mockSamlMeta: SamlMeta = {
  samlGeneric: mockSamlIdpServiceProvider,
};

export const MockSamlApplicationContextProvider = props => {
  const MockSamlApplicationContext = {
    runFetchMetadataValues: () => null,
    fetchMetadataValuesAttempt: {
      status: '',
      data: null,
      statusText: '',
    },
    upsertRequest: emptyUpsertRequest,
    setUpsertRequest: () => null,
    runUpsert: () => null,
    upsertAttempt: {
      status: '',
      data: null,
      statusText: '',
    },
    setPreset: () => null,
    guidedToggle: false,
    setGuidedToggle: () => null,
    guidedConfig: {},
    setGuidedConfig: () => null,
    onConfigChange: () => null,
  };
  const ctx = createTeleportContextE();
  return (
    <ContextProvider ctx={ctx}>
      <SamlApplicationProvider
        mockCtx={{
          ...MockSamlApplicationContext,
          ...props.samlProviderProps,
        }}
      >
        <Validation>{props.children}</Validation>
      </SamlApplicationProvider>
    </ContextProvider>
  );
};
