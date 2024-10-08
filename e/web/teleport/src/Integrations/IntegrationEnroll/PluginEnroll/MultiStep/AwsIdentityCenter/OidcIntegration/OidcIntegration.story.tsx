import { HttpResponse, delay, http } from 'msw';
import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { AwsIcOidcIntegration } from './OidcIntegration';

export default {
  title: 'TeleportE/Integrations/Enroll/AWSIdentityCenter/OidcIntegration',
};

export const ConfigureIntegration = () => {
  const ctx = createTeleportContextE();
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <AwsIcOidcIntegration />
      </ContextProvider>
    </MemoryRouter>
  );
};

ConfigureIntegration.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationsUrl(), () =>
        HttpResponse.json([
          {
            resourceType: 'integration',
            name: 'oidc1',
            kind: IntegrationKind.AwsOidc,
            statusCode: IntegrationStatusCode.Running,
            spec: { roleArn: '', issuerS3Bucket: '', issuerS3Prefix: '' },
          },
        ])
      ),
    ],
  },
};

export const ConfigureIntegrationLoading = () => {
  const ctx = createTeleportContextE();
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <AwsIcOidcIntegration />
      </ContextProvider>
    </MemoryRouter>
  );
};

ConfigureIntegrationLoading.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationsUrl(), async () => {
        await delay(5000);
        return HttpResponse.json([
          {
            resourceType: 'integration',
            name: 'oidc1',
            kind: IntegrationKind.AwsOidc,
            statusCode: IntegrationStatusCode.Running,
            spec: { roleArn: '', issuerS3Bucket: '', issuerS3Prefix: '' },
          },
        ]);
      }),
    ],
  },
};

export const ConfigureIntegrationFetchIntegrationFailed = () => {
  const ctx = createTeleportContextE();
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <AwsIcOidcIntegration />
      </ContextProvider>
    </MemoryRouter>
  );
};

ConfigureIntegrationFetchIntegrationFailed.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationsUrl(), () =>
        HttpResponse.json(
          {
            error: { message: 'Failed to fetch integrations' },
          },
          { status: 500 }
        )
      ),
    ],
  },
};
