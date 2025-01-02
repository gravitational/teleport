import { createMemoryHistory } from 'history';
import { Router } from 'react-router';

import { render, screen, userEvent } from 'design/utils/testing';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  AccessList,
  AccessListMemberKind,
  accessManagementService,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import ResourceService from 'teleport/services/resources';
import userService from 'teleport/services/user';

import { ViewEditAccessList } from './ViewEditAccessList';

let ctx: TeleportEContext;

beforeEach(() => {
  ctx = createTeleportContextE();

  jest
    .spyOn(accessManagementService, 'fetchAccessLists')
    .mockResolvedValue([mockAccessListApple]);
  jest
    .spyOn(accessManagementService, 'fetchAccessList')
    .mockResolvedValue(mockAccessListApple);
  jest
    .spyOn(ResourceService.prototype, 'fetchRoles')
    .mockResolvedValue({ items: [], startKey: '' });
  jest.spyOn(userService, 'fetchUsers').mockResolvedValue([]);
});

afterEach(() => {
  jest.resetAllMocks();
});

test('back button uses previous route if present and preserves queries', async () => {
  const history = createMemoryHistory({
    initialEntries: [`${cfg.getAccessListManagementRoute()}?search=banana`],
  });
  history.goBack = jest.fn();

  render(
    <Router history={history}>
      <ContextProvider ctx={ctx}>
        <AccessListManagementContextProvider>
          <ViewEditAccessList />
        </AccessListManagementContextProvider>
      </ContextProvider>
    </Router>
  );

  await screen.findByText(/apple/i);
  await userEvent.click(screen.getByTestId('back-button'));
  expect(history.goBack).toHaveBeenCalled();
  expect(history.location?.pathname).toBe(cfg.getAccessListManagementRoute());
  expect(history.location?.search).toBe('?search=banana');
});

test('back button uses default route if router state is not provided', async () => {
  const history = createMemoryHistory();
  history.push = jest.fn();
  // Manually unset location.key to simulate initial page load in-browser
  history.location.key = undefined;

  render(
    <Router history={history}>
      <ContextProvider ctx={ctx}>
        <AccessListManagementContextProvider>
          <ViewEditAccessList />
        </AccessListManagementContextProvider>
      </ContextProvider>
    </Router>
  );

  await screen.findByText(/apple/i);
  await userEvent.click(screen.getByTestId('back-button'));
  expect(history.push).toHaveBeenCalledWith(cfg.getAccessListManagementRoute());
});

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
