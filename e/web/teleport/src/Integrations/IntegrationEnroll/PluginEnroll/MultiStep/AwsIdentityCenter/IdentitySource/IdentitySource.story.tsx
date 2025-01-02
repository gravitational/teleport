import { MemoryRouter } from 'react-router';

import { PluginProvider } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/usePlugin';
import { pluginMap } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/plugins';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import type { CloudHostablePlugin } from 'e-teleport/services/plugins';
import { PluginConfigAwsIc } from 'e-teleport/services/plugins/types';
import { ContextProvider } from 'teleport';

import { AwsIcConfigureIdentitySource } from './IdentitySource';

export default {
  title: 'TeleportE/Integrations/Enroll/AWSIdentityCenter/IdentitySource',
};

const awsIdentityCenterPlugin = pluginMap[
  PluginConfigAwsIc.PluginName
] as CloudHostablePlugin;

export const ConfigureIdentitySource = () => {
  const ctx = createTeleportContextE();
  ctx.idpService.getMetadataXml = () =>
    Promise.resolve<string>('test file content');

  ctx.pluginsService.validatePlugin = () => Promise.resolve({ message: 'ok' });
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <PluginProvider selectedPlugin={awsIdentityCenterPlugin}>
          <AwsIcConfigureIdentitySource />
        </PluginProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

export const ConfigureIdentitySourceError = () => {
  const ctx = createTeleportContextE();
  ctx.idpService.getMetadataXml = () =>
    Promise.resolve<string>('test file content');
  ctx.pluginsService.validatePlugin = () =>
    Promise.reject({ message: 'service provider name already exists' });
  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <PluginProvider selectedPlugin={awsIdentityCenterPlugin}>
          <AwsIcConfigureIdentitySource />
        </PluginProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
