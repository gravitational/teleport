import { QueryClientProvider } from '@tanstack/react-query';
import { createMemoryHistory, History } from 'history';
import { delay, http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { Router } from 'react-router';

import {
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import { AccessListManagementContextProvider } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import {
  makeHandlers,
  unifiedResourcePath,
} from 'e-teleport/AccessListManagement/GuideEditor/Preset/TestHelper/mocks';
import cfg from 'e-teleport/config';
import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  AccessList,
  AccessListMemberKind,
  AccessListType,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import { ContextProvider } from 'teleport';

import { AccessListModified, modifyAccessList } from '../Shared';
import { DeleteAccessListConfirmDialog } from './DeleteAccessListConfirmDialog';

// msw matching requires stripping of query params
const rolesV2Path = cfg.oss.api.role.listV2.split('?')[0];

const server = setupServer(
  http.get(unifiedResourcePath, () => {
    return HttpResponse.json({ items: [] });
  }),
  ...makeHandlers()
);

beforeAll(() => {
  server.listen();
  jest.spyOn(console, 'warn').mockImplementation(() => {});
});

beforeEach(async () => {
  await testQueryClient.resetQueries();
});

afterEach(() => {
  jest.restoreAllMocks();
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});

test('shows loading state when fetching roles for preset access list', async () => {
  server.use(http.get(rolesV2Path, () => delay('infinite')));

  render(
    <Provider
      accessList={modifyAccessList({ ...baseAccessList, preset: 'long-term' })}
    />
  );

  await waitFor(() => {
    expect(screen.getByTestId('indicator')).toBeInTheDocument();
  });
});

test('shows error state when fetching roles fails with retry button for preset access list', async () => {
  jest.spyOn(console, 'error').mockImplementation(() => {});

  server.use(
    http.get(rolesV2Path, () => {
      return HttpResponse.json(
        { error: { message: 'Failed to fetch roles' } },
        { status: 500 }
      );
    })
  );

  render(
    <Provider
      accessList={modifyAccessList({ ...baseAccessList, preset: 'long-term' })}
    />
  );

  await screen.findByText(/failed to fetch roles/i);
  expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();

  // Clicking retry should attempt to fetch roles again - reset to success handler
  server.use(
    http.get(rolesV2Path, () => {
      return HttpResponse.json({ items: [], startKey: '' });
    })
  );
  await userEvent.click(screen.getByRole('button', { name: /retry/i }));

  // Should show delete content after successful retry
  await screen.findByText(/are you sure you want to delete/i);
});

test('shows delete confirmation for non-preset access list without fetching roles', async () => {
  let rolesFetched = false;
  server.use(
    http.get(rolesV2Path, () => {
      rolesFetched = true;
      return HttpResponse.json({ items: [], startKey: '' });
    })
  );

  render(<Provider accessList={modifyAccessList(baseAccessList)} />);

  // Should show delete confirmation immediately
  await screen.findByText(/are you sure you want to delete/i);
  expect(screen.getByText(/mocked title/i)).toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: /yes, delete access list/i })
  ).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();

  // Should not fetch roles for non-preset access lists
  expect(rolesFetched).toBe(false);
});

test('calls onClose when cancel button is clicked', async () => {
  const onClose = jest.fn();

  render(
    <Provider accessList={modifyAccessList(baseAccessList)} onClose={onClose} />
  );

  await screen.findByText(/are you sure you want to delete/i);
  await userEvent.click(screen.getByRole('button', { name: /cancel/i }));

  expect(onClose).toHaveBeenCalled();
});

test('shows error when delete fails', async () => {
  server.use(
    http.delete(cfg.api.accessListManagementPath, () => {
      return HttpResponse.json(
        { error: { message: 'Failed to delete access list' } },
        { status: 500 }
      );
    })
  );

  render(<Provider accessList={modifyAccessList(baseAccessList)} />);

  await screen.findByText(/are you sure you want to delete/i);
  await userEvent.click(
    screen.getByRole('button', { name: /yes, delete access list/i })
  );

  const errorMessage = await screen.findByText(/failed to delete access list/i);
  expect(errorMessage).toBeInTheDocument();
});

test('successfully deletes non-preset access list and redirects to access list management route', async () => {
  let deletedAccessListId: string | null = null;
  server.use(
    http.delete(cfg.api.accessListManagementPath, ({ params }) => {
      deletedAccessListId = params.accessListId as string;
      return HttpResponse.json({});
    })
  );

  const history = createMemoryHistory();

  render(
    <Provider accessList={modifyAccessList(baseAccessList)} history={history} />
  );

  await screen.findByText(/are you sure you want to delete/i);
  await userEvent.click(
    screen.getByRole('button', { name: /yes, delete access list/i })
  );

  await waitFor(() => {
    expect(deletedAccessListId).toBe('some-id-123');
  });

  // Should auto redirect.
  await waitFor(() => {
    expect(history.location.pathname).toBe(cfg.getAccessListManagementRoute());
  });
});

test('after deleting a preset access list, test listing and deleting from roles table', async () => {
  const deletedRoles: string[] = [];

  server.use(
    http.delete(cfg.api.accessListManagementPath, () => {
      return HttpResponse.json({});
    }),
    http.get(rolesV2Path, () => {
      return HttpResponse.json({
        items: [
          {
            name: 'role-one',
            object: {
              metadata: {
                name: 'role-one',
                labels: {
                  'teleport.internal/access-list-preset': 'some-id-123',
                },
              },
            },
          },
          {
            name: 'role-two',
            object: {
              metadata: {
                name: 'role-two',
                labels: {
                  'teleport.internal/access-list-preset': 'some-id-123',
                },
              },
            },
          },
        ],
        startKey: '',
      });
    }),
    http.delete(cfg.oss.api.role.delete, ({ params }) => {
      deletedRoles.push(params.name as string);
      return HttpResponse.json({});
    })
  );

  const history = createMemoryHistory();

  render(
    <Provider
      accessList={modifyAccessList({ ...baseAccessList, preset: 'long-term' })}
      history={history}
    />
  );

  // Wait for roles to be fetched and show delete confirmation
  await screen.findByText(/are you sure you want to delete/i);
  await userEvent.click(
    screen.getByRole('button', { name: /yes, delete access list/i })
  );

  // After delete, show roles table with all roles for deletion
  await screen.findByText(/successfully deleted access list/i);
  expect(screen.getByText('role-one')).toBeInTheDocument();
  expect(screen.getByText('role-two')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /close/i })).toBeInTheDocument();

  // Removing a role removes it from the table
  const removeButtons = screen.getAllByRole('button', { name: /remove/i });
  await userEvent.click(removeButtons[0]);

  // Verify deleteRole was called with the correct role name
  await waitFor(() => {
    expect(deletedRoles).toContain('role-one');
  });

  // Verify role is removed from table and other role remains
  await waitFor(() => {
    expect(screen.queryByText('role-one')).not.toBeInTheDocument();
  });
  expect(screen.getByText('role-two')).toBeInTheDocument();

  // Remove the remaining role.
  await userEvent.click(screen.getByRole('button', { name: /remove/i }));

  await waitFor(() => {
    expect(deletedRoles).toContain('role-two');
  });

  // Verify all roles were removed and empty state is shown
  await waitFor(() => {
    expect(screen.queryByText('role-two')).not.toBeInTheDocument();
  });
  expect(screen.getByText(/all roles deleted/i)).toBeInTheDocument();

  // Clicking Close button should auto redirect.
  await userEvent.click(screen.getByRole('button', { name: /close/i }));

  await waitFor(() => {
    expect(history.location.pathname).toBe(cfg.getAccessListManagementRoute());
  });
});

test('after deleting a preset access list with no roles to delete, redirects to access list management route', async () => {
  let accessListDeleted = false;

  server.use(
    http.delete(cfg.api.accessListManagementPath, () => {
      accessListDeleted = true;
      return HttpResponse.json({});
    }),
    http.get(rolesV2Path, () => {
      return HttpResponse.json({ items: [], startKey: '' });
    })
  );

  const history = createMemoryHistory();

  render(
    <Provider
      accessList={modifyAccessList({ ...baseAccessList, preset: 'long-term' })}
      history={history}
    />
  );

  await screen.findByText(/are you sure you want to delete/i);
  await userEvent.click(
    screen.getByRole('button', { name: /yes, delete access list/i })
  );

  await waitFor(() => {
    expect(accessListDeleted).toBe(true);
  });

  // Should auto redirect.
  await waitFor(() => {
    expect(history.location.pathname).toBe(cfg.getAccessListManagementRoute());
  });
});

const Provider = ({
  accessList,
  onClose = jest.fn(),
  history = createMemoryHistory(),
}: {
  accessList: AccessListModified;
  onClose?: () => void;
  history?: History;
}) => {
  const ctx = createTeleportContextE();
  return (
    <Router history={history}>
      <QueryClientProvider client={testQueryClient}>
        <ContextProvider ctx={ctx}>
          <AccessListManagementContextProvider>
            <DeleteAccessListConfirmDialog
              accessList={accessList}
              onClose={onClose}
            />
          </AccessListManagementContextProvider>
        </ContextProvider>
      </QueryClientProvider>
    </Router>
  );
};

const baseAccessList: AccessList = {
  id: 'some-id-123',
  metadata: {
    name: 'some-id-123',
    labels: {},
    revision: '',
  },
  type: AccessListType.Default,
  title: 'mocked title',
  description: 'some description',
  owners: [
    {
      name: 'owner-1',
      description: '',
      ineligibleReason: '',
      membershipKind: AccessListMemberKind.User,
    },
  ],
  members: [
    {
      name: 'member-1',
      joined: new Date(),
      expires: new Date(),
      addedBy: 'llama',
      membershipKind: AccessListMemberKind.User,
    },
  ],
  membersCount: 0,
  memberListCount: 0,
  grants: { roles: [''], traits: {} },
  ownerGrants: { roles: [''], traits: {} },
  audit: {
    recurrence: {
      frequency: ReviewFrequency.SixMonths,
      dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
    },
    nextDate: new Date(),
  },
  ownershipRequires: { roles: [], traits: {} },
  membershipRequires: { roles: [], traits: {} },
  inheritedMemberGrants: { roles: [], traits: {} },
};
