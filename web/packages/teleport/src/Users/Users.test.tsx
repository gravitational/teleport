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

import { MemoryRouter } from 'react-router';

import { fireEvent, render, screen, userEvent } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import { InfoGuidePanelProvider } from 'teleport/Main/InfoGuideContext';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { Access } from 'teleport/services/user';

import { Users } from './Users';
import { State } from './useUsers';

const defaultAcl: Access = {
  read: true,
  edit: true,
  remove: true,
  list: true,
  create: true,
};

describe('invite collaborators integration', () => {
  const ctx = createTeleportContext();

  let props: State;
  beforeEach(() => {
    props = {
      attempt: {
        message: 'success',
        isSuccess: true,
        isProcessing: false,
        isFailed: false,
      },
      users: [],
      fetchRoles: async () => [],
      operation: { type: 'invite-collaborators' },

      onStartCreate: () => undefined,
      onStartDelete: () => undefined,
      onStartEdit: () => undefined,
      onStartReset: () => undefined,
      onStartInviteCollaborators: () => undefined,
      onClose: () => undefined,
      onDelete: () => undefined,
      onCreate: () => undefined,
      onUpdate: () => undefined,
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
    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Users {...props} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    expect(screen.getByText('Create New User')).toBeInTheDocument();
    expect(screen.queryByText('Enroll Users')).not.toBeInTheDocument();
  });

  test('displays the Enroll Users button when configured', async () => {
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

    const enrollButton = screen.getByText('Enroll Users');
    expect(enrollButton).toBeInTheDocument();
    expect(screen.queryByText('Create New User')).not.toBeInTheDocument();

    enrollButton.click();
    expect(startMock.mock.calls).toHaveLength(1);

    // This will display regardless since the dialog display is managed by the
    // dialog itself, and our mock above is trivial, but we can make sure it
    // renders.
    expect(screen.getByTestId('invite-collaborators')).toBeInTheDocument();
  });
});

test('Users not equal to MAU Notice', async () => {
  const ctx = createTeleportContext();
  let props: State;

  props = {
    attempt: {
      message: 'success',
      isSuccess: true,
      isProcessing: false,
      isFailed: false,
    },
    users: [],
    fetchRoles: async () => [],
    operation: { type: 'invite-collaborators' },
    onStartCreate: () => undefined,
    onStartDelete: () => undefined,
    onStartEdit: () => undefined,
    onStartReset: () => undefined,
    onStartInviteCollaborators: () => undefined,
    onClose: () => undefined,
    onDelete: () => undefined,
    onCreate: () => undefined,
    onUpdate: () => undefined,
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

  expect(screen.getByTestId('users-not-mau-alert')).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'Dismiss' }));
  expect(props.onDismissUsersMauNotice).toHaveBeenCalled();
  expect(screen.queryByTestId('users-not-mau-alert')).not.toBeInTheDocument();
});

describe('email password reset integration', () => {
  const ctx = createTeleportContext();

  let props: State;
  beforeEach(() => {
    props = {
      attempt: {
        message: 'success',
        isSuccess: true,
        isProcessing: false,
        isFailed: false,
      },
      users: [],
      fetchRoles: () => Promise.resolve([]),
      operation: {
        type: 'reset',
        user: { name: 'alice@example.com', roles: ['foo'] },
      },

      onStartCreate: () => undefined,
      onStartDelete: () => undefined,
      onStartEdit: () => undefined,
      onStartReset: () => undefined,
      onStartInviteCollaborators: () => undefined,
      onClose: () => undefined,
      onDelete: () => undefined,
      onCreate: () => undefined,
      onUpdate: () => undefined,
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

  test('displays the email-based UI when configured', async () => {
    props = {
      ...props,
      InviteCollaborators: () => (
        <div data-testid="new-reset-ui">New Reset UI</div>
      ),
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

    expect(screen.getByText('New Reset UI')).toBeInTheDocument();

    // This will display regardless since the dialog display is managed by the
    // dialog itself, and our mock above is trivial, but we can make sure it
    // renders.
    expect(screen.getByTestId('new-reset-ui')).toBeInTheDocument();
  });
});

describe('permission handling', () => {
  const ctx = createTeleportContext();

  let props: State;
  beforeEach(() => {
    props = {
      attempt: {
        message: 'success',
        isSuccess: true,
        isProcessing: false,
        isFailed: false,
      },
      users: [
        {
          name: 'tester',
          roles: [],
          isLocal: true,
        },
      ],
      fetchRoles: () => Promise.resolve([]),
      operation: {
        type: 'reset',
        user: { name: 'alice@example.com', roles: ['foo'] },
      },

      onStartCreate: () => undefined,
      onStartDelete: () => undefined,
      onStartEdit: () => undefined,
      onStartReset: () => undefined,
      onStartInviteCollaborators: () => undefined,
      onClose: () => undefined,
      onDelete: () => undefined,
      onCreate: () => undefined,
      onUpdate: () => undefined,
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
