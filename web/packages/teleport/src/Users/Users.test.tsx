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

import React from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { Users } from './Users';
import { State } from './useUsers';

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
    };
  });

  test('displays the Create New User button when not configured', async () => {
    render(
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <Users {...props} />
        </ContextProvider>
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
        <ContextProvider ctx={ctx}>
          <Users {...props} />
        </ContextProvider>
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
    };
  });

  test('displays the traditional reset UI when not configured', async () => {
    render(
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <Users {...props} />
        </ContextProvider>
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
        <ContextProvider ctx={ctx}>
          <Users {...props} />
        </ContextProvider>
      </MemoryRouter>
    );

    expect(screen.getByText('New Reset UI')).toBeInTheDocument();

    // This will display regardless since the dialog display is managed by the
    // dialog itself, and our mock above is trivial, but we can make sure it
    // renders.
    expect(screen.getByTestId('new-reset-ui')).toBeInTheDocument();
  });
});
