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
import { MemoryRouter } from 'react-router';

import * as Icons from 'design/Icon';
import {
  render as testingRender,
  screen,
  fireEvent,
} from 'design/utils/testing';

import cfg from 'teleport/config';
import localStorage from 'teleport/services/localStorage';
import history from 'teleport/services/history';

import { UserMenuNav } from './UserMenuNav';

afterAll(() => {
  localStorage.clear();
});

describe('checkmark render', () => {
  test.each`
    viewing                                 | menuName
    ${cfg.routes.discover}                  | ${'Manage Access'}
    ${cfg.routes.users}                     | ${'Browse Resources'}
    ${cfg.getNodesRoute('some-cluster-id')} | ${'Browse Resources'}
    ${cfg.routes.account}                   | ${'Account'}
    ${cfg.routes.accountPassword}           | ${'Account'}
    ${cfg.routes.support}                   | ${'support'}
  `(
    'path `$viewing` renders checkmark next to menu item `$menuName`',
    ({ viewing, menuName }) => {
      render(viewing);

      // Click on dropdown menu.
      fireEvent.click(screen.getByText(/llama/i));

      // Only one checkmark should be rendered at a time.
      const targetEl = screen.getByTestId('checkmark');
      expect(targetEl).toBeInTheDocument();

      expect(targetEl.previousSibling).toHaveTextContent(menuName);
    }
  );
});

test('alert bubble rendered when there is no resources', () => {
  localStorage.setOnboardDiscover({ hasResource: false });
  render(cfg.routes.users);

  fireEvent.click(screen.getByText(/llama/i));

  const targetEl = screen.getByTestId('alert-bubble');
  expect(targetEl).toBeInTheDocument();

  expect(targetEl.parentNode.nextSibling).toHaveTextContent(/manage access/i);
});

test('alert bubble not rendered when viewing discovery', () => {
  localStorage.setOnboardDiscover({ hasResource: false });
  render(cfg.routes.discover);

  fireEvent.click(screen.getByText(/llama/i));

  const targetEl = screen.queryByTestId('alert-bubble');
  expect(targetEl).not.toBeInTheDocument();
});

test('alert bubble not rendered when there is resources', () => {
  localStorage.setOnboardDiscover({ hasResource: true });
  render(cfg.routes.discover);

  fireEvent.click(screen.getByText(/llama/i));

  const targetEl = screen.queryByTestId('alert-bubble');
  expect(targetEl).not.toBeInTheDocument();
});

test('clicking on discovery (going to) removes the alert bubble', () => {
  jest.spyOn(history, 'push').mockImplementation();
  localStorage.setOnboardDiscover({ hasResource: false });

  render(cfg.routes.users);

  fireEvent.click(screen.getByText(/llama/i));

  // Test initially we have the alert bubble when not viewing discovery.
  let targetEl = screen.getByTestId('alert-bubble');
  expect(targetEl).toBeInTheDocument();

  // Test clicking on discovery updates the local storage.
  fireEvent.click(screen.getByText(/manage access/i));
  expect(history.push).toHaveBeenCalledWith(cfg.routes.discover);
  expect(localStorage.getOnboardDiscover()).toEqual({
    hasResource: false,
    hasVisited: true,
  });

  // Test alert bubble is no longer rendered.
  fireEvent.click(screen.getByText(/llama/i));
  targetEl = screen.queryByTestId('alert-bubble');
  expect(targetEl).not.toBeInTheDocument();
});

function render(path: string) {
  testingRender(
    <MemoryRouter initialEntries={[path]}>
      <UserMenuNav
        navItems={[
          {
            title: 'support',
            Icon: Icons.Question,
            getLink() {
              return cfg.routes.support;
            },
          },
          {
            title: 'Account',
            Icon: Icons.Question,
            getLink() {
              return cfg.routes.account;
            },
          },
        ]}
        username="llama"
        logout={() => null}
      />
    </MemoryRouter>
  );
}
