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
import { render } from 'design/utils/testing';
import { UserAgent } from 'design/platform';

import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';

import {
  AllAccess,
  NoAccess,
  PartialAccess,
  InitRouteEntryServer,
} from './SelectResource.story';

beforeEach(() => {
  jest.restoreAllMocks();
  jest
    .spyOn(window.navigator, 'userAgent', 'get')
    .mockReturnValue(UserAgent.macOS);
});

test('render with all access', async () => {
  mockUserContextProviderWith(makeTestUserContext());
  const { container } = render(<AllAccess />);
  expect(container.firstChild).toMatchSnapshot();
});

test('render with no access', async () => {
  mockUserContextProviderWith(makeTestUserContext());
  const { container } = render(<NoAccess />);
  expect(container.firstChild).toMatchSnapshot();
});

test('render with partial access', async () => {
  mockUserContextProviderWith(makeTestUserContext());
  const { container } = render(<PartialAccess />);
  expect(container.firstChild).toMatchSnapshot();
});

test('render with URL loc state set to "server"', async () => {
  mockUserContextProviderWith(makeTestUserContext());
  const { container } = render(<InitRouteEntryServer />);
  expect(container.firstChild).toMatchSnapshot();
});
