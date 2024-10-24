import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';
import { Info } from 'design/Alert';

import { pluginMap } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/plugins';
import { PluginProvider } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/usePlugin';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { PluginConfigAwsIc } from 'e-teleport/services/plugins/types';

import { AwsIcConfigureScim } from './Scim';

import type { CloudHostablePlugin } from 'e-teleport/services/plugins';

export default {
  title: 'TeleportE/Integrations/Enroll/AWSIdentityCenter/SCIM',
};

const awsIdentityCenterPlugin = pluginMap[
  PluginConfigAwsIc.PluginName
] as CloudHostablePlugin;

export const ConfigureScim = () => {
  const ctx = createTeleportContextE();
  ctx.pluginsService.validatePlugin = () => Promise.resolve({ message: 'ok' });
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <PluginProvider selectedPlugin={awsIdentityCenterPlugin}>
          <AwsIcConfigureScim />
        </PluginProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

export const ConfigureScimError = () => {
  const ctx = createTeleportContextE();
  ctx.pluginsService.validatePlugin = () =>
    Promise.reject({ message: 'invalid credential' });
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <Info>
          Dev note: Enter valid https URL, random access token value and click
          Test SCIM button to view the SCIM validation error.
        </Info>
        <PluginProvider selectedPlugin={awsIdentityCenterPlugin}>
          <AwsIcConfigureScim />
        </PluginProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
