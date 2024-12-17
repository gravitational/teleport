import { createMemoryHistory } from 'history';
import { MemoryRouter, Router } from 'react-router';
import { render, screen, waitFor } from 'design/utils/testing';
import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { getAcl } from 'teleport/mocks/contexts';
import { ApiError } from 'teleport/services/api/parseError';

import ecfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  AccessList,
  AccessListMemberKind,
  accessManagementService,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import TeleportEContext from 'e-teleport/teleportContextE';
import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';

import { AccessLists } from './AccessLists';

const defaultIsEnterpriseFlag = cfg.isEnterprise;
const defaultAccessListEntitlement = cfg.entitlements.AccessLists;

describe('access list management upsell links', () => {
  const ctx = createTeleportContextE();

  beforeEach(() => {
    cfg.isEnterprise = true;

    jest
      .spyOn(accessManagementService, 'fetchAccessLists')
      .mockResolvedValue([]);
  });

  afterEach(() => {
    jest.resetAllMocks();

    cfg.isEnterprise = defaultIsEnterpriseFlag;
    cfg.entitlements.AccessLists = defaultAccessListEntitlement;
  });

  test('no access should not render cta', async () => {
    const error = new ApiError('', { status: 403 } as Response);

    jest
      .spyOn(accessManagementService, 'fetchAccessLists')
      .mockRejectedValue(error);

    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 0,
    };

    const ctx = createTeleportContextE({
      customAcl: getAcl({ noAccess: true }),
    });

    renderComponent(ctx);

    await waitFor(() => {
      expect(screen.queryByText('contact sales')).not.toBeInTheDocument();
    });
    expect(
      screen.getByText(/You do not have permission to view Access Lists/i)
    ).toBeInTheDocument();
  });

  test('unlimited access renders no cta', async () => {
    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 0,
    };

    renderComponent(ctx);

    await screen.findByText(/create your first access list/i);
    expect(screen.queryByText('contact sales')).not.toBeInTheDocument();
  });

  test('limited access renders cta', async () => {
    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 1,
    };

    renderComponent(ctx);

    await screen.findByText(/create your first access list/i);
    const link = screen.getByText(/contact sales/i);
    expect(link).toHaveAttribute('href', expect.stringMatching(/upgrade-igs/i));
  });
});

describe('access list management caching', () => {
  const ctx = createTeleportContextE();

  beforeEach(() => {
    cfg.isEnterprise = true;

    jest
      .spyOn(accessManagementService, 'fetchAccessLists')
      .mockResolvedValue([]);
  });

  afterEach(() => {
    jest.resetAllMocks();

    cfg.isEnterprise = defaultIsEnterpriseFlag;
    cfg.entitlements.AccessLists = defaultAccessListEntitlement;
  });

  test('if router state contains newly created access list, it is added to the items list', async () => {
    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 0,
    };
    jest
      .spyOn(accessManagementService, 'fetchAccessLists')
      .mockResolvedValue([mockAccessListApple]);

    const history = createMemoryHistory({
      initialEntries: [{ state: { createdList: mockAccessListBanana } }],
    });
    history.push = jest.fn();

    render(
      <Router history={history}>
        <ContextProvider ctx={ctx}>
          <AccessListManagementContextProvider>
            <AccessLists />
          </AccessListManagementContextProvider>
        </ContextProvider>
      </Router>
    );

    await screen.findByText(/apple/i);
    expect(screen.getByText(/banana/i)).toBeInTheDocument();
  });

  test('if router state contains newly created access list, is is NOT duplicated if it already exists in items list', async () => {
    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 0,
    };
    jest
      .spyOn(accessManagementService, 'fetchAccessLists')
      .mockResolvedValue([mockAccessListApple, mockAccessListBanana]);

    const history = createMemoryHistory({
      initialEntries: [{ state: { createdList: mockAccessListBanana } }],
    });
    history.push = jest.fn();

    render(
      <Router history={history}>
        <ContextProvider ctx={ctx}>
          <AccessListManagementContextProvider>
            <AccessLists />
          </AccessListManagementContextProvider>
        </ContextProvider>
      </Router>
    );

    await screen.findByText(/apple/i);
    expect(screen.getByText(/banana/i)).toBeInTheDocument();
  });

  test('if router state contains deleted access list ID, it is removed from the items list', async () => {
    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 0,
    };
    jest
      .spyOn(accessManagementService, 'fetchAccessLists')
      .mockResolvedValue([mockAccessListApple, mockAccessListBanana]);

    const history = createMemoryHistory({
      initialEntries: [{ state: { deletedAccessListId: 'id-banana' } }],
    });
    history.push = jest.fn();

    render(
      <Router history={history}>
        <ContextProvider ctx={ctx}>
          <AccessListManagementContextProvider>
            <AccessLists />
          </AccessListManagementContextProvider>
        </ContextProvider>
      </Router>
    );

    await screen.findByText(/apple/i);
    expect(screen.queryByText(/banana/i)).not.toBeInTheDocument();
  });

  test('if router state contains reviewed access list, notification item is rendered and review by badge is not rendered', async () => {
    jest.useFakeTimers();
    jest.setSystemTime(new Date('2023-01-20'));
    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 0,
    };
    jest.spyOn(accessManagementService, 'fetchAccessLists').mockResolvedValue([
      {
        ...mockAccessListApple,
        // due "today"
        audit: { ...mockAccessListApple.audit, nextDate: new Date() },
      },
    ]);

    // Test review by date badge is rendered.
    const { unmount } = render(
      <Router history={createMemoryHistory()}>
        <ContextProvider ctx={ctx}>
          <AccessListManagementContextProvider>
            <AccessLists />
          </AccessListManagementContextProvider>
        </ContextProvider>
      </Router>
    );

    await screen.findByText(/apple/i);
    expect(screen.getByText(/review by 01\/20/i)).toBeInTheDocument();
    expect(screen.queryByText(/submitted review/i)).not.toBeInTheDocument();
    unmount();

    // Now render with a location state.

    const history = createMemoryHistory({
      initialEntries: [
        {
          state: {
            reviewedAccessList: {
              ...mockAccessListApple,
              audit: {
                ...mockAccessListApple.audit,
                nextDate: new Date('2023-12-25'),
              },
            },
          },
        },
      ],
    });

    render(
      <Router history={history}>
        <ContextProvider ctx={ctx}>
          <AccessListManagementContextProvider>
            <AccessLists />
          </AccessListManagementContextProvider>
        </ContextProvider>
      </Router>
    );

    await screen.findByText(/submitted review for "apple"/i);
    expect(screen.queryByText(/review by/i)).not.toBeInTheDocument();

    jest.useRealTimers();
  });

  test('search param is respected', async () => {
    ecfg.oss.entitlements.AccessLists = {
      enabled: true,
      limit: 0,
    };
    jest
      .spyOn(accessManagementService, 'fetchAccessLists')
      .mockResolvedValue([mockAccessListApple, mockAccessListBanana]);

    const history = createMemoryHistory({
      initialEntries: [{ pathname: 'web/random', search: '?search=bana' }],
    });
    history.push = jest.fn();

    render(
      <Router history={history}>
        <ContextProvider ctx={ctx}>
          <AccessListManagementContextProvider>
            <AccessLists />
          </AccessListManagementContextProvider>
        </ContextProvider>
      </Router>
    );

    await screen.findByText(/banana/i);
    expect(screen.queryByText(/apple/i)).not.toBeInTheDocument();
  });
});

function renderComponent(ctx: TeleportEContext) {
  return render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <AccessListManagementContextProvider>
          <AccessLists />
        </AccessListManagementContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}

const mockAccessListApple: AccessList = {
  id: 'id-apple',
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

const mockAccessListBanana: AccessList = {
  id: 'id-banana',
  title: 'banana',
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
