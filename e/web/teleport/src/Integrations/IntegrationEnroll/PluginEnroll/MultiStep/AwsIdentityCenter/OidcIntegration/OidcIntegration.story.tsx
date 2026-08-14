import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import ecfg from 'e-teleport/config';
import {
  DevNoteOidc,
  integrationsResponse,
  integrationsResponseWithAwsIcAudience,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/AwsIdentityCenter/shared/fixture';
import { PluginProvider } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/usePlugin';
import { pluginMap } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/plugins';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import type { CloudHostablePlugin } from 'e-teleport/services/plugins';
import { PluginConfigAwsIc } from 'e-teleport/services/plugins/types';
import { ContextProvider } from 'teleport';

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

ConfigureIntegration.beforeEach = ({ msw }) => {
  msw.use(
    http.get(ecfg.oss.getIntegrationsUrl(), () =>
      HttpResponse.json(integrationsResponse)
    ),
    http.post(ecfg.oss.getIntegrationsUrl(), () =>
      HttpResponse.json(integrationsResponse.items[0])
    ),
    http.post(ecfg.getPluginValidateUrl(), () =>
      HttpResponse.json({ message: 'ok' })
    )
  );
};

export const ExistingIntegration = () => {
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

ExistingIntegration.beforeEach = ({ msw }) => {
  msw.use(
    http.get(ecfg.oss.getIntegrationsUrl(), () =>
      HttpResponse.json(integrationsResponseWithAwsIcAudience)
    )
  );
};

export const Loading = () => {
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

Loading.beforeEach = ({ msw }) => {
  msw.use(
    http.get(ecfg.oss.getIntegrationsUrl(), async () => {
      await delay(5000);
      return HttpResponse.json(integrationsResponse);
    })
  );
};

export const FetchIntegrationFailed = () => {
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

FetchIntegrationFailed.beforeEach = ({ msw }) => {
  msw.use(
    http.get(ecfg.oss.getIntegrationsUrl(), () =>
      HttpResponse.json(
        {
          error: { message: 'Failed to fetch integrations' },
        },
        { status: 500 }
      )
    ),
    http.post(ecfg.oss.getIntegrationsUrl(), () =>
      HttpResponse.json(
        {
          error: { message: 'Failed to create new integration' },
        },
        { status: 500 }
      )
    )
  );
};

export const CreateIntegrationError = () => {
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

CreateIntegrationError.beforeEach = ({ msw }) => {
  msw.use(
    http.get(ecfg.oss.getIntegrationsUrl(), () =>
      HttpResponse.json(integrationsResponse)
    ),
    http.post(ecfg.oss.getIntegrationsUrl(), () =>
      HttpResponse.json(
        {
          error: { message: 'Failed to create new integration' },
        },
        { status: 500 }
      )
    )
  );
};

export const CredentialValidationFailed = () => {
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

CredentialValidationFailed.beforeEach = ({ msw }) => {
  msw.use(
    http.get(ecfg.oss.getIntegrationsUrl(), () =>
      HttpResponse.json(integrationsResponseWithAwsIcAudience)
    ),
    http.post(ecfg.getPluginValidateUrl(), () =>
      HttpResponse.json(
        {
          error: {
            message: `unauthorized`,
            response: { status: 401 } as Response,
          },
        },
        { status: 401 }
      )
    )
  );
};

export const CredentialValidationFailedWithNotFoundError = () => {
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

CredentialValidationFailedWithNotFoundError.beforeEach = ({ msw }) => {
  msw.use(
    http.get(ecfg.oss.getIntegrationsUrl(), () =>
      HttpResponse.json(integrationsResponseWithAwsIcAudience)
    ),
    http.post(ecfg.getPluginValidateUrl(), () =>
      HttpResponse.json(
        {
          error: {
            message: `integration "aws-oidc" doesn't exist`,
            response: { status: 404 } as Response,
          },
        },
        { status: 404 }
      )
    )
  );
};
