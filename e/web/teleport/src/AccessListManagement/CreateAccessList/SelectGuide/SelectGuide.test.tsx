import { QueryClientProvider } from '@tanstack/react-query';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { http, HttpResponse } from 'msw';

import {
  enableMswServer,
  screen,
  server,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { mockAccessLists } from 'e-teleport/AccessListManagement/AccessLists/EmptyState/fixtures';
import { unifiedResourcePath } from 'e-teleport/AccessListManagement/GuideEditor/Preset/TestHelper/mocks';
import ecfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { accessManagementService } from 'e-teleport/services/accessmanagement';
import { pluginsService } from 'e-teleport/services/plugins';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { allAccessAcl, getAcl, noAccess } from 'teleport/mocks/contexts';
import type { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import type { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';
import ResourceService from 'teleport/services/resources';
import userService from 'teleport/services/user';
import { userEventService } from 'teleport/services/userEvent';
import { renderWithMemoryRouter } from 'teleport/test/helpers/router';
import { UserContextProvider } from 'teleport/User';

import { CreateAccessList } from '../CreateAccessList';
import { CreateAccessListContextProvider } from '../CreateAccessListContextProvider';
import { SelectGuide } from './SelectGuide';

const defaultIsEnterpriseFlag = cfg.isEnterprise;
const defaultAccessListentitlement = cfg.entitlements.AccessLists;
const rootScopedRolesPath = ecfg.getRootScopedRolesUrl({}).split('?')[0];

enableMswServer();
mockIntersectionObserver();

beforeEach(() => {
  server.use(
    http.get(unifiedResourcePath, () => {
      return HttpResponse.json({
        items: [],
      });
    }),
    http.get(cfg.api.userPreferencesPath, () => {
      return HttpResponse.json({});
    }),
    http.get(rootScopedRolesPath, () => {
      return HttpResponse.json({
        roles: [],
        startKey: '',
      });
    }),
    // Typing an access list title schedules a debounced terraform config request that can fire after the test
    // that typed it has finished, so it is handled here rather than per test.
    http.post(ecfg.getAccessListWithPresetUrl({ action: 'terraform' }), () => {
      return HttpResponse.json({ terraform: '' });
    })
  );
});

afterEach(async () => {
  await testQueryClient.resetQueries();
});

describe('upsell links', () => {
  const ctx = createTeleportContextE();

  beforeEach(() => {
    cfg.isEnterprise = true;

    jest
      .spyOn(accessManagementService, 'fetchAccessListsV2')
      .mockResolvedValue({ agents: [mockAccessLists[0]] });

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

  test('no rbac access should not render tiles', async () => {
    ecfg.oss.entitlements.AccessLists = {
      // license is enabled, but RBAC access says its denied
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

    await waitFor(() => {
      expect(screen.queryByText(/contact sales/i)).not.toBeInTheDocument();
    });
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

  test('both buttons are enabled with full role access', async () => {
    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 0,
    };

    jest
      .spyOn(userEventService, 'captureAccessListEvent')
      .mockImplementation(() => {});

    const user = userEvent.setup();

    const { unmount } = renderWithRoleAccess(allAccessAcl.roles);
    await screen.findByText(/Select a guide/i);

    expect(screen.getByRole('button', { name: /start guide/i })).toBeEnabled();
    expect(
      screen.getByRole('button', { name: /use custom form instead/i })
    ).toBeEnabled();

    // Test clicking into start guide button
    await user.type(
      screen.getByPlaceholderText(/Access List name/i),
      'guided title'
    );
    await user.click(screen.getByRole('button', { name: /start guide/i }));
    expect(
      await screen.findByText(/define access to resources/i)
    ).toBeInTheDocument();

    // Unmount before the debounced terraform request can resolve into a mounted tree outside act().
    unmount();
  });

  test('only custom button is enabled with no role access', async () => {
    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 0,
    };

    jest
      .spyOn(userEventService, 'captureAccessListEvent')
      .mockImplementation(() => {});

    const user = userEvent.setup();

    renderWithRoleAccess(noAccess);
    await screen.findByText(/Select a guide/i);

    expect(screen.getByRole('button', { name: /start guide/i })).toBeDisabled();
    expect(
      screen.getByRole('button', { name: /use custom form instead/i })
    ).toBeEnabled();

    // Test clicking into custom button
    await user.type(
      screen.getByPlaceholderText(/Access List name/i),
      'some title'
    );
    await user.click(
      screen.getByRole('button', { name: /use custom form instead/i })
    );
    expect(await screen.findByText(/basic information/i)).toBeInTheDocument();
  });
});

function renderWithRoleAccess(roles: typeof noAccess) {
  return renderComponent(
    createTeleportContextE({
      customAcl: {
        ...allAccessAcl,
        roles,
      },
    }),
    <CreateAccessList />
  );
}

function renderComponent(ctx: TeleportEContext, component = <SelectGuide />) {
  return renderWithMemoryRouter(
    <QueryClientProvider client={testQueryClient}>
      <InfoGuidePanelProvider>
        <ContextProvider ctx={ctx}>
          <UserContextProvider>
            <AccessListManagementContextProvider>
              <CreateAccessListContextProvider>
                {component}
              </CreateAccessListContextProvider>
            </AccessListManagementContextProvider>
          </UserContextProvider>
        </ContextProvider>
      </InfoGuidePanelProvider>
    </QueryClientProvider>,
    {
      initialEntries: [ecfg.routes.accessListNew],
    }
  );
}
