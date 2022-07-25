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

import { render, screen, act, fireEvent } from 'design/utils/testing';

import { DiscoverContext } from '../discoverContext';
import ContextProvider from '../discoverContextProvider';

import LoginTrait from './LoginTrait';

import type { User } from 'teleport/services/user';
import type { RenderResult } from '@testing-library/react';

describe('login trait comp behavior', () => {
  const ctx = new DiscoverContext();
  const userSvc = ctx.userService;

  let Component;

  beforeEach(() => {
    Component = (
      <ContextProvider value={ctx}>
        <LoginTrait
          // TODO we don't need all of this
          attempt={null}
          joinToken={null}
          createJoinToken={null}
          agentMeta={null}
          updateAgentMeta={null}
          nextStep={null}
          prevStep={null}
        />
      </ContextProvider>
    );
  });

  test('add a new login', async () => {
    jest.spyOn(userSvc, 'fetchUser').mockResolvedValue(mockUser);

    let r: RenderResult;
    await act(async () => {
      r = render(Component);
    });

    // Expect no checkboxes to be rendered.
    const checkboxes = r.container.querySelectorAll('input[type=checkbox]');
    expect(checkboxes).toHaveLength(0);

    // Test adding a new login name.
    fireEvent.click(screen.getByText(/add new/i));
    const inputEl = screen.getByPlaceholderText('name');
    fireEvent.change(inputEl, { target: { value: 'banana' } });
    fireEvent.click(screen.getByText('Add'));
    expect(screen.getByText('banana')).toBeInTheDocument();
  });

  test('rendering of init logins', async () => {
    jest.spyOn(userSvc, 'fetchUser').mockResolvedValue({
      ...mockUser,
      traits: {
        ...mockUser.traits,
        logins: ['apple', 'banana'],
      },
    });

    let r: RenderResult;
    await act(async () => {
      r = render(Component);
    });

    // Test init logins to be rendered.
    let checkboxes: NodeListOf<HTMLInputElement> =
      r.container.querySelectorAll('input:checked');
    expect(checkboxes).toHaveLength(2);
    expect(checkboxes[0].name).toBe('apple');
    expect(checkboxes[1].name).toBe('banana');

    // Test init logins to be rendered with a new login name.
    fireEvent.click(screen.getByText(/add new/i));
    const inputEl = screen.getByPlaceholderText('name');
    fireEvent.change(inputEl, { target: { value: 'carrot' } });
    fireEvent.click(screen.getByText('Add'));

    checkboxes = r.container.querySelectorAll('input:checked');
    expect(checkboxes).toHaveLength(3);
    expect(checkboxes[0].name).toBe('apple');
    expect(checkboxes[1].name).toBe('banana');
    expect(checkboxes[2].name).toBe('carrot');
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
