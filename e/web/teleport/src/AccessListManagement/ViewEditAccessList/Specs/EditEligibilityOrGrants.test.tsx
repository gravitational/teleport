import selectEvent from 'react-select-event';

import { act, render, screen, userEvent, waitFor } from 'design/utils/testing';

import {
  AccessList,
  AccessListOrigin,
  AccessListType,
  accessManagementService,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import ResourceService from 'teleport/services/resources';

import { modifyAccessList } from '../Shared';
import { EditEligibilityOrGrantRoles } from './EditEligibilityOrGrants';

beforeEach(() => {
  jest
    .spyOn(ResourceService.prototype, 'fetchRoles')
    .mockResolvedValue({ items: [], startKey: '' });
  jest
    .spyOn(accessManagementService, 'fetchRootScopedRoles')
    .mockResolvedValue({
      roles: [
        {
          name: 'team-admin',
          scope: '/',
          assignableScopes: ['/dev/**'],
        },
      ],
      startKey: '',
    });
});

afterEach(() => {
  jest.resetAllMocks();
});

test('saves edited scoped roles for member grants', async () => {
  const user = userEvent.setup();
  const onClose = jest.fn();
  const updateAccessList = jest.fn();
  const updatedAccessList = makeAccessList();

  const updateSpy = jest
    .spyOn(accessManagementService, 'updateAccessList')
    .mockResolvedValue(updatedAccessList);

  render(
    <EditEligibilityOrGrantRoles
      accessList={modifyAccessList(makeAccessList())}
      editKind="Grants"
      onClose={onClose}
      updateAccessList={updateAccessList}
    />
  );

  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: 'Add a Scoped Role Grant' })
    ).toBeEnabled();
  });

  await user.click(
    screen.getByRole('button', { name: 'Add a Scoped Role Grant' })
  );

  await act(async () => {
    await selectEvent.select(
      screen.getByLabelText('Scoped Role Name'),
      '/::team-admin'
    );
  });
  const scopeInput = screen.getByLabelText('Assigned Scope');
  await user.type(scopeInput, '/dev/api');
  expect(scopeInput).toHaveValue('/dev/api');
  await waitFor(() => {
    expect(
      screen.queryByText('Scope must match one of: /dev/**')
    ).not.toBeInTheDocument();
  });

  await user.click(
    screen.getByRole('button', { name: 'Save Permissions Granted' })
  );

  await waitFor(() => {
    expect(updateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        req: expect.objectContaining({
          grants: {
            roles: [],
            traits: {},
            scopedRoles: [{ role: '/::team-admin', scope: '/dev/api' }],
          },
        }),
      })
    );
  });

  expect(onClose).toHaveBeenCalled();
  expect(updateAccessList).toHaveBeenCalledWith(updatedAccessList);
});

test('keeps scope input focused while typing', async () => {
  const user = userEvent.setup();

  render(
    <EditEligibilityOrGrantRoles
      accessList={modifyAccessList(makeAccessList())}
      editKind="Grants"
      onClose={jest.fn()}
      updateAccessList={jest.fn()}
    />
  );

  await waitFor(() => {
    expect(
      screen.getByRole('button', { name: 'Add a Scoped Role Grant' })
    ).toBeEnabled();
  });

  await user.click(
    screen.getByRole('button', { name: 'Add a Scoped Role Grant' })
  );
  await act(async () => {
    await selectEvent.select(
      screen.getByLabelText('Scoped Role Name'),
      '/::team-admin'
    );
  });

  const scopeInput = screen.getByLabelText('Assigned Scope');
  await user.click(scopeInput);
  await user.type(scopeInput, '/dev/api');

  expect(screen.getByLabelText('Assigned Scope')).toHaveValue('/dev/api');
  expect(screen.getByLabelText('Assigned Scope')).toHaveFocus();
});

function makeAccessList(): AccessList {
  return {
    id: 'access-list-id',
    type: AccessListType.Default,
    metadata: {
      name: 'access-list-id',
      labels: {},
      revision: '1',
    },
    title: 'Test Access List',
    description: 'test description',
    owners: [],
    members: [],
    grants: { roles: [], traits: {}, scopedRoles: [] },
    ownerGrants: { roles: [], traits: {}, scopedRoles: [] },
    inheritedMemberGrants: { roles: [], traits: {}, scopedRoles: [] },
    ownershipRequires: { roles: [], traits: {} },
    membershipRequires: { roles: [], traits: {} },
    audit: {
      recurrence: {
        dayOfMonth: ReviewDayOfMonth.FirstDayOfMonth,
        frequency: ReviewFrequency.OneMonth,
      },
      nextDate: new Date('2026-01-01T00:00:00Z'),
    },
    origin: AccessListOrigin.Unspecified,
    preset: '',
    currentUserAssignments: undefined,
    userAssignments: undefined,
    membersCount: 0,
    memberListCount: 0,
  };
}
