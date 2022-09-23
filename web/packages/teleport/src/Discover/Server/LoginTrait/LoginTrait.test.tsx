/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { fireEvent, render, screen, waitFor } from 'design/utils/testing';

import { MemoryRouter } from 'react-router';

import TeleportContext from 'teleport/teleportContext';
import ContextProvider from 'teleport/TeleportContextProvider';
import makeAcl from 'teleport/services/user/makeAcl';

import { ResourceKind } from 'teleport/Discover/Shared';

import LoginTrait from './LoginTrait';

import type { User, UserContext } from 'teleport/services/user';
import type { NodeMeta } from '../../useDiscover';

describe('login trait comp behavior', () => {
  const setup = (mockUserContext: Partial<UserContext>, mockUser: User) => {
    const ctx = new TeleportContext();
    ctx.storeUser.setState(mockUserContext);
    const userSvc = ctx.userService;

    jest.spyOn(userSvc, 'fetchUser').mockResolvedValue(mockUser);
    jest.spyOn(userSvc, 'updateUser').mockResolvedValue(null);
    jest.spyOn(userSvc, 'applyUserTraits').mockResolvedValue(null);

    render(
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <LoginTrait
            agentMeta={mockedNodeMeta}
            updateAgentMeta={() => null}
            nextStep={() => null}
            selectedResourceKind={ResourceKind.Server}
            onSelectResource={() => null}
          />
        </ContextProvider>
      </MemoryRouter>
    );

    return { userSvc };
  };

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('add a new login with no existing logins', async () => {
    setup(userContext, mockUser);

    // Expect no checkboxes to be rendered.
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();

    // Test adding a new login name.
    fireEvent.click(await screen.findByText(/add new OS/i));
    const inputEl = screen.getByPlaceholderText('name');
    fireEvent.change(inputEl, { target: { value: 'banana' } });
    fireEvent.click(screen.getByText('Add'));
    expect(screen.getByText('banana')).toBeInTheDocument();
  });

  test('add a new login with existing logins', async () => {
    const { userSvc } = setup(userContext, mockUserWithLoginTrait);

    // Test existing logins to be rendered.
    await waitFor(() => {
      expect(screen.getAllByRole('checkbox')).toHaveLength(2);
    });
    expect(screen.getByLabelText('apple')).toBeChecked();
    expect(screen.getByLabelText('banana')).toBeChecked();

    // Test existing logins to be rendered with a new login name.
    fireEvent.click(screen.getByText(/add new OS/i));
    const inputEl = screen.getByPlaceholderText('name');
    fireEvent.change(inputEl, { target: { value: 'carrot' } });
    fireEvent.click(screen.getByText('Add'));

    expect(screen.getAllByRole('checkbox')).toHaveLength(3);
    expect(screen.getByLabelText('apple')).toBeChecked();
    expect(screen.getByLabelText('banana')).toBeChecked();
    expect(screen.getByLabelText('carrot')).toBeChecked();

    fireEvent.click(screen.getByText(/next/i));
    expect(userSvc.updateUser).toHaveBeenCalledWith({
      ...mockUserWithLoginTrait,
      traits: {
        ...mockUserWithLoginTrait.traits,
        logins: [...mockUserWithLoginTrait.traits.logins, 'carrot'],
      },
    });
  });

  test('skipping api calls without perms', async () => {
    const { userSvc } = setup(
      { ...userContext, acl: makeAcl({ users: { edit: false } }) },
      mockUserWithLoginTrait
    );
    await waitFor(() => {
      expect(screen.getAllByRole('checkbox')).toHaveLength(2);
    });
    fireEvent.click(screen.getByText(/next/i));
    expect(userSvc.updateUser).not.toHaveBeenCalled();
  });

  test('skipping api calls when user used sso', async () => {
    const { userSvc } = setup(
      { ...userContext, authType: 'sso' },
      mockUserWithLoginTrait
    );
    await waitFor(() => {
      expect(screen.getAllByRole('checkbox')).toHaveLength(2);
    });
    fireEvent.click(screen.getByText(/next/i));
    expect(userSvc.updateUser).not.toHaveBeenCalled();
  });
});

const mockUser: User = {
  name: 'foo',
  roles: [],
  traits: {
    logins: [],
    databaseUsers: [],
    databaseNames: [],
    kubeUsers: [],
    kubeGroups: [],
    windowsLogins: [],
    awsRoleArns: [],
  },
};

const mockUserWithLoginTrait: User = {
  ...mockUser,
  traits: {
    ...mockUser.traits,
    logins: ['apple', 'banana'],
  },
};

const mockedNodeMeta: NodeMeta = {
  resourceName: 'resource-name',
  node: {
    sshLogins: [],
    id: '',
    clusterId: '',
    hostname: '',
    labels: [],
    addr: '',
    tunnel: false,
  },
};

const userContext = {
  acl: makeAcl({ users: { edit: true } }),
  authType: 'local' as const,
};
