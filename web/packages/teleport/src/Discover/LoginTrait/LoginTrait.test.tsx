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

import { render, screen, fireEvent, waitFor } from 'design/utils/testing';

import { MemoryRouter } from 'react-router';

import TeleportContext from 'teleport/teleportContext';
import ContextProvider from 'teleport/TeleportContextProvider';

import LoginTrait from './LoginTrait';

import type { User } from 'teleport/services/user';
import type { NodeMeta } from '../useDiscover';

describe('login trait comp behavior', () => {
  const ctx = new TeleportContext();
  const userSvc = ctx.userService;

  const Component = () => (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <LoginTrait
          // TODO we don't need all of this
          attempt={null}
          joinToken={null}
          createJoinToken={null}
          agentMeta={mockedNodeMeta}
          updateAgentMeta={null}
          nextStep={null}
          prevStep={null}
        />
      </ContextProvider>
    </MemoryRouter>
  );

  test('add a new login with no existing logins', async () => {
    jest.spyOn(userSvc, 'fetchUser').mockResolvedValue(mockUser);

    render(<Component />);

    // Expect no checkboxes to be rendered.
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();

    // Test adding a new login name.
    fireEvent.click(await screen.findByText(/add an/i));
    const inputEl = screen.getByPlaceholderText('name');
    fireEvent.change(inputEl, { target: { value: 'banana' } });
    fireEvent.click(screen.getByText('Add'));
    expect(screen.getByText('banana')).toBeInTheDocument();
  });

  test('add a new login with existing logins', async () => {
    jest.spyOn(userSvc, 'fetchUser').mockResolvedValue({
      ...mockUser,
      traits: {
        ...mockUser.traits,
        logins: ['apple', 'banana'],
      },
    });

    render(<Component />);

    // Test existing logins to be rendered.
    await waitFor(() => {
      expect(screen.getAllByRole('checkbox')).toHaveLength(2);
    });
    expect(screen.getByLabelText('apple')).toBeChecked();
    expect(screen.getByLabelText('banana')).toBeChecked();

    // Test existing logins to be rendered with a new login name.
    fireEvent.click(screen.getByText(/add an OS/i));
    const inputEl = screen.getByPlaceholderText('name');
    fireEvent.change(inputEl, { target: { value: 'carrot' } });
    fireEvent.click(screen.getByText('Add'));

    expect(screen.getAllByRole('checkbox')).toHaveLength(3);
    expect(screen.getByLabelText('apple')).toBeChecked();
    expect(screen.getByLabelText('banana')).toBeChecked();
    expect(screen.getByLabelText('carrot')).toBeChecked();
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

const mockedNodeMeta: NodeMeta = {
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
