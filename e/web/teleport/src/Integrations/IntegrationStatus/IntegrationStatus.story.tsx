import { StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { Route, Switch } from 'teleport/components/Router';
import { ContextProvider } from 'teleport/index';

import { IntegrationStatus } from './IntegrationStatus';

export default {
  title: 'TeleportE/Integrations/Status',
};

export const EntraID: StoryObj = {
  beforeEach({ msw }) {
    msw.use(
      http.get(cfg.api.plugin.get, () => {
        return HttpResponse.json({
          resourceType: 'plugin',
          type: 'entra-id',
          name: 'entra-id-default',
          statusCode: 1,
          spec: {
            defaultOwners: ['emaster', 'user@example.com', 'admin@example.com'],
            accessListOwnersSource: 'ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN',
            ssoConnectorId: 'entra-id',
            credentialSource: 'ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS',
            tenantId: '71cbeb2a-1b5b-44bd-909f-511a904a25b0',
            entraAppId: '9a5b7068-bb3c-4f76-9183-c0fbb6483b15',
            groupFilters: {
              nameRegex: ['admin*', 'devops-prod*'],
              excludeId: [
                'a80b5881-d483-4724-8298-7a36017d6f94',
                'a80b5881-d483-4724-8298-7a36017d6f94',
              ],
            },
            accessGraphEnabled: true,
          },
          status: {
            code: 1,
            lastRun: new Date('2025-12-11T10:50:47.910628Z'),
            errorMessage: '',
            details: {
              entra: {
                imported_users: 600,
                imported_groups: 200,
                sync_mode: 'delta',
              },
            },
          },
        });
      })
    );
  },

  render: () => {
    cfg.oss.entitlements.AccessGraph = { enabled: true, limit: 0 };
    return render(
      cfg.oss.getIntegrationStatusRoute('entra-id', 'entra-id-default')
    );
  },
};

const render = (pathname: string) => {
  const ctx = createTeleportContextE();

  return (
    <MemoryRouter initialEntries={[{ pathname }]}>
      <ContextProvider ctx={ctx}>
        <Switch>
          <Route path={cfg.oss.routes.integrationStatus}>
            <IntegrationStatus />
          </Route>
        </Switch>
      </ContextProvider>
    </MemoryRouter>
  );
};
