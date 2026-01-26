import { QueryClientProvider } from '@tanstack/react-query';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { MemoryRouter } from 'react-router';

import {
  act,
  Providers,
  render,
  screen,
  testQueryClient,
  waitFor,
} from 'design/utils/testing';
import { ToastNotificationProvider } from 'shared/components/ToastNotification';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import ecfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  AccessList,
  AccessListMemberKind,
  AccessListType,
  accessManagementService,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import { pluginsService } from 'e-teleport/services/plugins';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { getAcl } from 'teleport/mocks/contexts';
import { ApiError } from 'teleport/services/api/parseError';
import type { Plugin } from 'teleport/services/integrations';

import { unifiedResourcePath } from '../GuideEditor/Preset/TestHelper/mocks';
import { AccessLists } from './AccessLists';

const mio = mockIntersectionObserver();
const defaultIsEnterpriseFlag = cfg.isEnterprise;
const defaultAccessListEntitlement = cfg.entitlements.AccessLists;

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

describe('access list management upsell links', () => {
  const ctx = createTeleportContextE();

  beforeEach(() => {
    cfg.isEnterprise = true;

    jest
      .spyOn(accessManagementService, 'fetchAccessListsV2')
      .mockResolvedValue({ agents: [], startKey: '' });
    jest.spyOn(pluginsService, 'fetchPlugin').mockResolvedValue({} as Plugin);
  });

  afterEach(async () => {
    jest.resetAllMocks();

    cfg.isEnterprise = defaultIsEnterpriseFlag;
    cfg.entitlements.AccessLists = defaultAccessListEntitlement;
    testQueryClient.clear();
    await testQueryClient.resetQueries();
  });

  test('no access should not render cta', async () => {
    const error = new ApiError({
      message: '',
      response: { status: 403 } as Response,
    });

    jest
      .spyOn(accessManagementService, 'fetchAccessListsV2')
      .mockRejectedValue(error);

    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 0,
    };

    const ctx = createTeleportContextE({
      customAcl: getAcl({ noAccess: true }),
    });

    renderComponent(ctx);
    act(mio.enterAll);

    await waitFor(() => {
      expect(screen.queryByText('contact sales')).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(
        screen.getByText(/You do not have permission to view Access Lists/i)
      ).toBeInTheDocument();
    });
    expect(screen.getByText(/What are Access Lists/i)).toBeInTheDocument();
  });

  test('unlimited access renders no cta', async () => {
    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 0,
    };

    renderComponent(ctx);
    act(mio.enterAll);

    await screen.findByText(/create your first access list/i);
    expect(screen.queryByText('contact sales')).not.toBeInTheDocument();
  });

  test('limited access renders cta', async () => {
    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 1,
    };

    renderComponent(ctx);
    act(mio.enterAll);

    await screen.findByText(/create your first access list/i);
    const link = screen.getByText(/contact sales/i);
    expect(link).toHaveAttribute('href', expect.stringMatching(/upgrade-igs/i));
  });
});

function renderComponent(ctx: TeleportEContext) {
  return render(
    <QueryClientProvider client={testQueryClient}>
      <Providers>
        <MemoryRouter>
          <ToastNotificationProvider>
            <ContextProvider ctx={ctx}>
              <AccessListManagementContextProvider>
                <AccessLists />
              </AccessListManagementContextProvider>
            </ContextProvider>
          </ToastNotificationProvider>
        </MemoryRouter>
      </Providers>
    </QueryClientProvider>
  );
}

const mockAccessListApple: AccessList = {
  id: 'id-apple',
  type: AccessListType.Default,
  title: 'apple',
  description: '',
  owners: [
    {
      name: 'lisa',
      description: '',
      ineligibleReason: '',
      membershipKind: AccessListMemberKind.User,
    },
  ],
  members: [],
  membersCount: 0,
  memberListCount: 0,
  grants: { roles: ['access'], traits: {} },
  ownerGrants: { roles: [], traits: {} },
  audit: {
    recurrence: {
      frequency: ReviewFrequency.SixMonths,
      dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
    },
    nextDate: new Date('2024-06-08T07:00:00.000Z'),
  },
  ownershipRequires: { roles: [], traits: {} },
  membershipRequires: { roles: [], traits: {} },
  inheritedMemberGrants: { roles: [], traits: {} },
};

test(`should show access list if backend returns it, even if user lacks list and read permission`, async () => {
  jest
    .spyOn(accessManagementService, 'fetchAccessListsV2')
    .mockResolvedValue({ agents: [mockAccessListApple] });
  jest.spyOn(pluginsService, 'fetchPlugin').mockResolvedValue({} as Plugin);
  const ctx = createTeleportContextE({
    customAcl: getAcl({ noAccess: true }),
  });

  renderComponent(ctx);
  act(mio.enterAll);

  await waitFor(() => {
    expect(screen.getByText(/apple/i)).toBeInTheDocument();
  });
});
