import React from 'react';
import { createMemoryHistory } from 'history';
import { Router } from 'react-router';
import { render, screen, userEvent } from 'design/utils/testing';
import { ContextProvider } from 'teleport';
import ResourceService from 'teleport/services/resources';
import userService from 'teleport/services/user';

import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  AccessList,
  accessManagementService,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import TeleportEContext from 'e-teleport/teleportContextE';

import { ViewEditAccessList } from './ViewEditAccessList';

let ctx: TeleportEContext;

beforeEach(() => {
  ctx = createTeleportContextE();

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

test('back button uses router provided state "previousPath', async () => {
  const history = createMemoryHistory({
    initialEntries: [{ state: { previousPath: 'web/random?search=banana' } }],
  });
  history.push = jest.fn();

  render(
    <Router history={history}>
      <ContextProvider ctx={ctx}>
        <ViewEditAccessList />
      </ContextProvider>
    </Router>
  );

  await screen.findByText(/apple/i);
  await userEvent.click(screen.getByTestId('back-button'));
  expect(history.push).toHaveBeenCalledWith('web/random?search=banana');
});

test('back button uses default route if router state is not provided', async () => {
  const history = createMemoryHistory();
  history.push = jest.fn();

  render(
    <Router history={history}>
      <ContextProvider ctx={ctx}>
        <ViewEditAccessList />
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
  owners: [{ name: 'lisa', description: '', ineligibleReason: '' }],
  members: [],
  membersCount: 0,
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
};
