/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
