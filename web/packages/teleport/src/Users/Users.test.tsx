/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { setupServer } from 'msw/node';
import { MemoryRouter } from 'react-router';

import {
  fireEvent,
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { Access } from 'teleport/services/user';
import { successGetUsersV2 } from 'teleport/test/helpers/users';

import { Users } from './Users';
import { State } from './useUsers';

const defaultAcl: Access = {
  read: true,
  edit: true,
  remove: true,
  list: true,
  create: true,
};

const server = setupServer();

beforeEach(() => server.listen());
afterEach(() => {
  server.resetHandlers();

  return testQueryClient.resetQueries();
});
afterAll(() => server.close());

describe('invite collaborators integration', () => {
  const ctx = createTeleportContext();

  let props: State;
  beforeEach(() => {
    props = {
      operation: { type: 'invite-collaborators' },
      fetch: ctx.userService.fetchUsersV2,
      onStartCreate: () => undefined,
      onStartDelete: () => undefined,
      onStartEdit: () => undefined,
      onStartReset: () => undefined,
      onStartInviteCollaborators: () => undefined,
      onClose: () => undefined,
      onReset: () => undefined,
      onInviteCollaboratorsClose: () => undefined,
      InviteCollaborators: null,
      inviteCollaboratorsOpen: false,
      onEmailPasswordResetClose: () => undefined,
      EmailPasswordReset: null,
      showMauInfo: false,
      onDismissUsersMauNotice: () => null,
      usersAcl: defaultAcl,
    };
  });

  test('displays the Create New User button when not configured', async () => {
    server.use(successGetUsersV2([]));

    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Users {...props} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    await screen.findByPlaceholderText('Search...');
    await waitFor(() => {
      expect(screen.getByText('Create New User')).toBeInTheDocument();
    });
    expect(screen.queryByText('Enroll Users')).not.toBeInTheDocument();
  });

  test('displays the Enroll Users button when configured', async () => {
    server.use(successGetUsersV2([]));

    const startMock = jest.fn();
    props = {
      ...props,
      InviteCollaborators: () => (
        <div data-testid="invite-collaborators">Invite Collaborators</div>
      ),
      onStartInviteCollaborators: startMock,
    };

    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Users {...props} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    await screen.findByPlaceholderText('Search...');

    const enrollButton = await screen.findByText('Enroll Users');
    expect(enrollButton).toBeInTheDocument();
    expect(screen.queryByText('Create New User')).not.toBeInTheDocument();

    const user = userEvent.setup();
    await user.click(enrollButton);
    expect(startMock.mock.calls).toHaveLength(1);

    // Ensure the passed in component for InviteCollaborators renders.
    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Users {...props} inviteCollaboratorsOpen={true} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );
    expect(screen.getByTestId('invite-collaborators')).toBeInTheDocument();
  });
});

test('Users not equal to MAU Notice', async () => {
  server.use(successGetUsersV2([]));

  const ctx = createTeleportContext();
  let props: State;

  props = {
    operation: { type: 'invite-collaborators' },
    fetch: ctx.userService.fetchUsersV2,
    onStartCreate: () => undefined,
    onStartDelete: () => undefined,
    onStartEdit: () => undefined,
    onStartReset: () => undefined,
    onStartInviteCollaborators: () => undefined,
    onClose: () => undefined,
    onReset: () => undefined,
    onInviteCollaboratorsClose: () => undefined,
    InviteCollaborators: null,
    inviteCollaboratorsOpen: false,
    onEmailPasswordResetClose: () => undefined,
    EmailPasswordReset: null,
    showMauInfo: true,
    onDismissUsersMauNotice: jest.fn(),
    usersAcl: defaultAcl,
  };

  const user = userEvent.setup();

  render(
    <MemoryRouter>
      <InfoGuidePanelProvider>
        <ContextProvider ctx={ctx}>
          <Users {...props} />
        </ContextProvider>
      </InfoGuidePanelProvider>
    </MemoryRouter>
  );

  await screen.findByPlaceholderText('Search...');

  const alert = await screen.findByTestId('users-not-mau-alert');
  expect(alert).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: 'Dismiss' }));

  expect(props.onDismissUsersMauNotice).toHaveBeenCalled();
  expect(screen.queryByTestId('users-not-mau-alert')).not.toBeInTheDocument();
});

describe('email password reset integration', () => {
  const ctx = createTeleportContext();

  let props: State;
  beforeEach(() => {
    server.use(successGetUsersV2([]));

    props = {
      operation: {
        type: 'reset',
        user: { name: 'alice@example.com', roles: ['foo'] },
      },
      fetch: ctx.userService.fetchUsersV2,
      onStartCreate: () => undefined,
      onStartDelete: () => undefined,
      onStartEdit: () => undefined,
      onStartReset: () => undefined,
      onStartInviteCollaborators: () => undefined,
      onClose: () => undefined,
      onReset: () => undefined,
      onInviteCollaboratorsClose: () => undefined,
      InviteCollaborators: null,
      inviteCollaboratorsOpen: false,
      onEmailPasswordResetClose: () => undefined,
      EmailPasswordReset: null,
      showMauInfo: false,
      onDismissUsersMauNotice: () => null,
      usersAcl: defaultAcl,
    };
  });

  test('displays the traditional reset UI when not configured', async () => {
    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Users {...props} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    expect(screen.getByText('Reset User Authentication?')).toBeInTheDocument();
    expect(screen.queryByText('New Reset UI')).not.toBeInTheDocument();
  });
});

describe('permission handling', () => {
  const ctx = createTeleportContext();

  let props: State;
  beforeEach(() => {
    server.use(
      successGetUsersV2([
        {
          name: 'tester',
          roles: [],
          authType: 'local',
        },
      ])
    );

    props = {
      operation: {
        type: 'reset',
        user: { name: 'alice@example.com', roles: ['foo'] },
      },
      fetch: ctx.userService.fetchUsersV2,
      onStartCreate: () => undefined,
      onStartDelete: () => undefined,
      onStartEdit: () => undefined,
      onStartReset: () => undefined,
      onStartInviteCollaborators: () => undefined,
      onClose: () => undefined,
      onReset: () => undefined,
      onInviteCollaboratorsClose: () => undefined,
      InviteCollaborators: null,
      inviteCollaboratorsOpen: false,
      onEmailPasswordResetClose: () => undefined,
      EmailPasswordReset: null,
      showMauInfo: false,
      onDismissUsersMauNotice: () => null,
      usersAcl: defaultAcl,
    };
  });

  test('displays a disabled Create Users button if lacking permissions', async () => {
    const testProps = {
      ...props,
      usersAcl: {
        ...defaultAcl,
        edit: false,
      },
    };
    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Users {...testProps} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    await screen.findByPlaceholderText('Search...');

    expect(screen.getByTestId('create_new_users_button')).toBeDisabled();
  });

  test('edit and reset options not available in the menu', async () => {
    const testProps = {
      ...props,
      usersAcl: {
        ...defaultAcl,
        edit: false,
      },
    };
    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Users {...testProps} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    await screen.findByPlaceholderText('Search...');

    const optionsButton = screen.getByRole('button', { name: /options/i });
    fireEvent.click(optionsButton);
    const menuItems = screen.queryAllByRole('menuitem');
    expect(menuItems).toHaveLength(1);
    expect(menuItems.some(item => item.textContent.includes('Delete'))).toBe(
      true
    );
  });

  test('all options are available in the menu', async () => {
    const testProps = {
      ...props,
      usersAcl: {
        read: true,
        list: true,
        edit: true,
        create: true,
        remove: true,
      },
    };
    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Users {...testProps} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    await screen.findByPlaceholderText('Search...');

    expect(screen.getByText('tester')).toBeInTheDocument();
    const optionsButton = screen.getByRole('button', { name: /options/i });
    fireEvent.click(optionsButton);
    const menuItems = screen.queryAllByRole('menuitem');
    expect(menuItems).toHaveLength(3);
    expect(menuItems.some(item => item.textContent.includes('Delete'))).toBe(
      true
    );
    expect(
      menuItems.some(item => item.textContent.includes('Reset Auth'))
    ).toBe(true);
    expect(menuItems.some(item => item.textContent.includes('Edit'))).toBe(
      true
    );
  });

  test('delete is not available in menu', async () => {
    const testProps = {
      ...props,
      usersAcl: {
        read: true,
        list: true,
        edit: true,
        create: true,
        remove: false,
      },
    };
    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Users {...testProps} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    await screen.findByPlaceholderText('Search...');

    expect(screen.getByText('tester')).toBeInTheDocument();
    const optionsButton = screen.getByRole('button', { name: /options/i });
    fireEvent.click(optionsButton);
    const menuItems = screen.queryAllByRole('menuitem');
    expect(menuItems).toHaveLength(2);
    expect(
      menuItems.every(item => item.textContent.includes('Delete'))
    ).not.toBe(true);
  });
});
