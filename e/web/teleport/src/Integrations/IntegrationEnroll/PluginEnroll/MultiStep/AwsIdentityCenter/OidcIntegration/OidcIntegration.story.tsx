import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import {
  DevNoteOidc,
  integrationsResponse,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/AwsIdentityCenter/shared/fixture';
import { PluginProvider } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/usePlugin';
import { pluginMap } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/plugins';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import type { CloudHostablePlugin } from 'e-teleport/services/plugins';
import { PluginConfigAwsIc } from 'e-teleport/services/plugins/types';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';

import { AwsIcOidcIntegration } from './OidcIntegration';

export default {
  title: 'TeleportE/Integrations/Enroll/AWSIdentityCenter/OidcIntegration',
};

const awsIdentityCenterPlugin = pluginMap[
  PluginConfigAwsIc.PluginName
] as CloudHostablePlugin;

export const ConfigureIntegration = () => {
  const ctx = createTeleportContextE();
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <PluginProvider selectedPlugin={awsIdentityCenterPlugin}>
          <AwsIcOidcIntegration />
        </PluginProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

ConfigureIntegration.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationsUrl(), () =>
        HttpResponse.json(integrationsResponse)
      ),
    ],
  },
};

export const ConfigureIntegrationExistingIntegration = () => {
  const ctx = createTeleportContextE();
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <PluginProvider selectedPlugin={awsIdentityCenterPlugin}>
          <AwsIcOidcIntegration />
        </PluginProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

ConfigureIntegrationExistingIntegration.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationsUrl(), () =>
        HttpResponse.json({
          items: [
            {
              name: 'new-integration',
              subKind: 'aws-oidc',
              awsoidc: {
                roleArn: 'arn:aws:iam::026090554232:role/new-integration',
                audience: 'aws-identity-center',
              },
            },
          ],
          nextKey: '',
        })
      ),
    ],
  },
};

export const ConfigureIntegrationLoading = () => {
  const ctx = createTeleportContextE();
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <PluginProvider selectedPlugin={awsIdentityCenterPlugin}>
          <AwsIcOidcIntegration />
        </PluginProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

ConfigureIntegrationLoading.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationsUrl(), async () => {
        await delay(5000);
        return HttpResponse.json(integrationsResponse);
      }),
    ],
  },
};

export const ConfigureIntegrationFetchIntegrationFailed = () => {
  const ctx = createTeleportContextE();
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <PluginProvider selectedPlugin={awsIdentityCenterPlugin}>
          <AwsIcOidcIntegration />
        </PluginProvider>
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
      http.post(cfg.getIntegrationsUrl(), () =>
        HttpResponse.json(
          {
            error: { message: 'Failed to create new integration' },
          },
          { status: 500 }
        )
      ),
    ],
  },
};

export const ConfigureIntegrationCreateIntegrationError = () => {
  const ctx = createTeleportContextE();
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <PluginProvider selectedPlugin={awsIdentityCenterPlugin}>
          <DevNoteOidc />
          <AwsIcOidcIntegration />
        </PluginProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

ConfigureIntegrationCreateIntegrationError.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getIntegrationsUrl(), () =>
        HttpResponse.json(integrationsResponse)
      ),
      http.post(cfg.getIntegrationsUrl(), () =>
        HttpResponse.json(
          {
            error: { message: 'Failed to create new integration' },
          },
          { status: 500 }
        )
      ),
    ],
  },
};
