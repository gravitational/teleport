import { QueryClientProvider } from '@tanstack/react-query';
import { addWeeks } from 'date-fns';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import {
  act,
  enableMswServer,
  Providers,
  render,
  screen,
  server,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';
import { AccessListUserAssignmentType } from 'gen-proto-ts/teleport/accesslist/v1/accesslist_pb';
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

enableMswServer();

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
  await testQueryClient.resetQueries();
});

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
  metadata: {
    name: 'id-apple',
    labels: {},
    revision: '',
  },
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
  grants: { roles: ['access'], traits: {}, scopedRoles: [] },
  ownerGrants: { roles: [], traits: {}, scopedRoles: [] },
  audit: {
    recurrence: {
      frequency: ReviewFrequency.SixMonths,
      dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
    },
    nextDate: new Date('2024-06-08T07:00:00.000Z'),
  },
  ownershipRequires: { roles: [], traits: {} },
  membershipRequires: { roles: [], traits: {} },
  inheritedMemberGrants: { roles: [], traits: {}, scopedRoles: [] },
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

test('static access list with zero next audit date does not render review UI in list view', async () => {
  jest.spyOn(accessManagementService, 'fetchAccessListsV2').mockResolvedValue({
    agents: [
      {
        ...mockAccessListApple,
        id: 'id-static-list',
        metadata: {
          ...mockAccessListApple.metadata,
          name: 'id-static-list',
        },
        type: AccessListType.Static,
        title: 'static list',
        audit: {
          ...mockAccessListApple.audit,
          nextDate: new Date('0001-01-01T00:00:00Z'),
        },
      },
    ],
  });
  jest.spyOn(pluginsService, 'fetchPlugin').mockResolvedValue({} as Plugin);

  renderComponent(createTeleportContextE());
  await userEvent.click(screen.getByRole('radio', { name: 'List View' }));
  act(mio.enterAll);

  await screen.findByText('static list');
  expect(screen.queryByText('0001-01-01')).not.toBeInTheDocument();
  expect(screen.queryByText(/review now/i)).not.toBeInTheDocument();
});

describe('review badge visibility', () => {
  const reviewDueDate = addWeeks(new Date(), 1);

  const makeReviewableList = (
    overrides: Partial<AccessList> = {}
  ): AccessList => ({
    ...mockAccessListApple,
    audit: {
      ...mockAccessListApple.audit,
      nextDate: reviewDueDate,
    },
    ...overrides,
  });

  beforeEach(() => {
    jest.spyOn(pluginsService, 'fetchPlugin').mockResolvedValue({} as Plugin);
  });

  afterEach(async () => {
    jest.resetAllMocks();
    testQueryClient.clear();
    await testQueryClient.resetQueries();
  });

  test('owner sees review badge when review is due', async () => {
    const list = makeReviewableList({
      currentUserAssignments: {
        ownershipType: AccessListUserAssignmentType.EXPLICIT,
        membershipType: AccessListUserAssignmentType.UNSPECIFIED,
      },
    });
    jest
      .spyOn(accessManagementService, 'fetchAccessListsV2')
      .mockResolvedValue({ agents: [list] });

    renderComponent(createTeleportContextE());
    act(mio.enterAll);

    await waitFor(() => {
      expect(screen.getByText(/review now/i)).toBeInTheDocument();
    });
  });

  test('owner who is also a member sees review badge', async () => {
    const list = makeReviewableList({
      currentUserAssignments: {
        ownershipType: AccessListUserAssignmentType.EXPLICIT,
        membershipType: AccessListUserAssignmentType.EXPLICIT,
      },
      membersCount: undefined,
    });
    jest
      .spyOn(accessManagementService, 'fetchAccessListsV2')
      .mockResolvedValue({ agents: [list] });

    renderComponent(createTeleportContextE());
    act(mio.enterAll);

    await waitFor(() => {
      expect(screen.getByText(/review now/i)).toBeInTheDocument();
    });
  });

  test('pure member without admin perms does not see review badge', async () => {
    const list = makeReviewableList({
      currentUserAssignments: {
        ownershipType: AccessListUserAssignmentType.UNSPECIFIED,
        membershipType: AccessListUserAssignmentType.EXPLICIT,
      },
      membersCount: undefined,
    });
    jest
      .spyOn(accessManagementService, 'fetchAccessListsV2')
      .mockResolvedValue({ agents: [list] });

    renderComponent(
      createTeleportContextE({ customAcl: getAcl({ noAccess: true }) })
    );
    act(mio.enterAll);

    await waitFor(() => {
      expect(screen.getByText(/apple/i)).toBeInTheDocument();
    });
    expect(screen.queryByText(/review now/i)).not.toBeInTheDocument();
  });

  test('review badge not shown when review is not due', async () => {
    const list = makeReviewableList({
      currentUserAssignments: {
        ownershipType: AccessListUserAssignmentType.EXPLICIT,
        membershipType: AccessListUserAssignmentType.UNSPECIFIED,
      },
      audit: {
        ...mockAccessListApple.audit,
        nextDate: addWeeks(new Date(), 4),
      },
    });
    jest
      .spyOn(accessManagementService, 'fetchAccessListsV2')
      .mockResolvedValue({ agents: [list] });

    renderComponent(createTeleportContextE());
    act(mio.enterAll);

    await waitFor(() => {
      expect(screen.getByText(/apple/i)).toBeInTheDocument();
    });
    expect(screen.queryByText(/review now/i)).not.toBeInTheDocument();
  });

  test('admin with edit permission sees review badge', async () => {
    // No currentUserAssignments (not an owner), but has admin edit permission.
    const list = makeReviewableList();
    jest
      .spyOn(accessManagementService, 'fetchAccessListsV2')
      .mockResolvedValue({ agents: [list] });

    renderComponent(createTeleportContextE());
    act(mio.enterAll);

    await waitFor(() => {
      expect(screen.getByText(/review now/i)).toBeInTheDocument();
    });
  });
});
