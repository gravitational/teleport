import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { IntegrationStatusCode, Plugin } from 'teleport/services/integrations';

import { EditGroupsImport } from './GroupsImport';

export default {
  title: 'TeleportE/Integrations/Enroll/Entra/Edit/GroupsImport',

  beforeEach({ msw }) {
    msw.use(
      http.get(cfg.getUsersUrlV2(), () => {
        return HttpResponse.json({
          items: [{ name: 'alice' }, { name: 'bob' }, { name: 'carol' }],
          startKey: '',
        });
      })
    );
  },
};

export const Default = () => {
  return (
    <MemoryRouter>
      <ContextProvider ctx={createTeleportContextE()}>
        <EditGroupsImport disabled={false} onSave={() => null} />
      </ContextProvider>
    </MemoryRouter>
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
      accessListOwnersSource:
        'ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN_AND_ENTRAID',
    },
  };
  return (
    <MemoryRouter>
      <ContextProvider ctx={createTeleportContextE()}>
        <EditGroupsImport
          plugin={plugin}
          disabled={false}
          onSave={() => null}
        />
      </ContextProvider>
    </MemoryRouter>
  );
};
