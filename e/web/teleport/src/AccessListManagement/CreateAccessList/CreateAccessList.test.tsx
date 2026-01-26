import { QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { MemoryRouter } from 'react-router';

import { render, screen, testQueryClient } from 'design/utils/testing';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { mockAccessLists } from 'e-teleport/AccessListManagement/AccessLists/EmptyState/fixtures';
import ecfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { accessManagementService } from 'e-teleport/services/accessmanagement';
import { pluginsService } from 'e-teleport/services/plugins';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { getAcl } from 'teleport/mocks/contexts';
import type { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import type { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';
import ResourceService from 'teleport/services/resources';
import userService from 'teleport/services/user';

import { unifiedResourcePath } from '../GuideEditor/Preset/TestHelper/mocks';
import { CreateAccessList } from './CreateAccessList';
import { CreateAccessListContextProvider } from './CreateAccessListContextProvider';

const defaultIsEnterpriseFlag = cfg.isEnterprise;
const defaultAccessListentitlement = cfg.entitlements.AccessLists;

const server = setupServer();

beforeAll(() => {
  server.listen();
});

beforeEach(() => {
  server.use(
    http.get(unifiedResourcePath, () => {
      return HttpResponse.json({
        items: [],
      });
    })
  );
});

afterEach(async () => {
  server.resetHandlers();
  await testQueryClient.resetQueries();
});

afterAll(() => server.close());

describe('upsell links', () => {
  const ctx = createTeleportContextE();

  beforeEach(() => {
    cfg.isEnterprise = true;

    jest
      .spyOn(accessManagementService, 'fetchAccessListsV2')
      .mockResolvedValue({ agents: [mockAccessLists[0]] });

    jest.spyOn(userService, 'fetchUsers').mockResolvedValue([]);
    jest
      .spyOn(userService, 'fetchUsersV2')
      .mockResolvedValue({ startKey: '', items: [] });
    jest.spyOn(ResourceService.prototype, 'fetchRoles').mockResolvedValue({
      items: [],
      startKey: '',
    });
    jest
      .spyOn(pluginsService, 'fetchPlugin')
      .mockResolvedValue({} as Plugin<PluginOktaSpec, PluginStatusOkta>);
  });

  afterEach(() => {
    jest.resetAllMocks();

    cfg.isEnterprise = defaultIsEnterpriseFlag;
    cfg.entitlements.AccessLists = defaultAccessListentitlement;
  });

  test('no access should not render cta', async () => {
    ecfg.oss.entitlements.AccessLists = {
      // access denied via ACL
      enabled: true,
      limit: 0,
    };

    const ctx = createTeleportContextE({
      customAcl: getAcl({ noAccess: true }),
    });

    renderComponent(ctx);

    await screen.findByText(
      /Only Teleport administrators can create new Access Lists/i
    );
    expect(screen.queryByText('contact sales')).not.toBeInTheDocument();
  });

  test('unlimited & enabled entitlement renders no cta', async () => {
    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 0,
    };

    renderComponent(ctx);

    await screen.findByText(/title/i);

    expect(screen.queryByText(/contact sales/i)).not.toBeInTheDocument();
  });

  test('limited entitlement renders cta', async () => {
    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 1,
    };

    renderComponent(ctx);

    const link = await screen.findByText(/contact sales/i);
    expect(link.parentElement).toHaveAttribute(
      'href',
      expect.stringMatching(/upgrade-igs/i)
    );
  });
});

function renderComponent(ctx: TeleportEContext) {
  return render(
    <MemoryRouter>
      <QueryClientProvider client={testQueryClient}>
        <ContextProvider ctx={ctx}>
          <AccessListManagementContextProvider>
            <CreateAccessListContextProvider>
              <CreateAccessList />
            </CreateAccessListContextProvider>
          </AccessListManagementContextProvider>
        </ContextProvider>
      </QueryClientProvider>
    </MemoryRouter>
  );
}
