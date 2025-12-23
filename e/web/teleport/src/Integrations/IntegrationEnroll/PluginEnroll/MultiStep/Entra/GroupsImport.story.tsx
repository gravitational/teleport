import { http, HttpResponse } from 'msw';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { IntegrationStatusCode, Plugin } from 'teleport/services/integrations';

import { EditGroupsImport } from './GroupsImport';

export default {
  title: 'TeleportE/Integrations/Enroll/Entra/Edit/GroupsImport',
  parameters: {
    msw: {
      handlers: [
        http.get(cfg.getUsersUrlV2(), () => {
          return HttpResponse.json({
            items: [{ name: 'alice' }, { name: 'bob' }, { name: 'carol' }],
            startKey: '',
          });
        }),
      ],
    },
  },
};

export const Default = () => {
  return (
    <ContextProvider ctx={createTeleportContextE()}>
      <EditGroupsImport />
    </ContextProvider>
  );
};

export const PrefillFromPluginSpec = () => {
  const plugin: Plugin = {
    kind: 'entra-id',
    name: 'entra-id-default',
    resourceType: 'plugin',
    statusCode: IntegrationStatusCode.Running,
    spec: {
      defaultOwners: ['alice', 'bob'],
      groupFilters: { id: ['abc123', 'abc456'], excludeNameRegex: ['admin*'] },
    },
  };
  return (
    <ContextProvider ctx={createTeleportContextE()}>
      <EditGroupsImport plugin={plugin} />
    </ContextProvider>
  );
};
