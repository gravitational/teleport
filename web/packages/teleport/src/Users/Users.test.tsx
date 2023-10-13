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

import React, { useState } from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen } from 'design/utils/testing';

import { useAttempt } from 'shared/hooks';

import { getOSSFeatures } from 'teleport/features';
import { User } from 'teleport/services/user';
import { Context, ContextProvider } from 'teleport';
import { events } from 'teleport/Audit/fixtures';
import { clusters } from 'teleport/Clusters/fixtures';
import { nodes } from 'teleport/Nodes/fixtures';
import { sessions } from 'teleport/Sessions/fixtures';
import { apps } from 'teleport/Apps/fixtures';
import { kubes } from 'teleport/Kubes/fixtures';
import { databases } from 'teleport/Databases/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';
import { userContext } from 'teleport/Main/fixtures';
import { LayoutContextProvider } from 'teleport/Main/LayoutContext';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';
import TeleportContext from 'teleport/teleportContext';

import { Users } from './Users';
import { Operation, State } from './useUsers';

jest.mock('shared/hooks', () => ({
  useAttempt: () => {
    return {
      attempt: { status: 'success', statusText: 'Success Text' },
      setAttempt: jest.fn(),
      run: (fn?: any) => Promise.resolve(fn()),
    };
  },
}));

const setupContext = (): TeleportContext => {
  const ctx = new Context();
  ctx.isEnterprise = false;
  ctx.auditService.fetchEvents = () =>
    Promise.resolve({ events, startKey: '' });
  ctx.clusterService.fetchClusters = () => Promise.resolve(clusters);
  ctx.nodeService.fetchNodes = () => Promise.resolve({ agents: nodes });
  ctx.sshService.fetchSessions = () => Promise.resolve(sessions);
  ctx.appService.fetchApps = () => Promise.resolve({ agents: apps });
  ctx.kubeService.fetchKubernetes = () => Promise.resolve({ agents: kubes });
  ctx.databaseService.fetchDatabases = () =>
    Promise.resolve({ agents: databases });
  ctx.desktopService.fetchDesktops = () =>
    Promise.resolve({ agents: desktops });
  ctx.storeUser.setState(userContext);

  return ctx;
};

describe('InviteCollaborators', () => {
  mockUserContextProviderWith(makeTestUserContext());
  const ctx = setupContext();

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
      roles: [],
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

    expect(screen.queryByText('Create New User')).toBeInTheDocument();
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
    expect(screen.queryByTestId('invite-collaborators')).toBeInTheDocument();
  });
});
