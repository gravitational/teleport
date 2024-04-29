/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import {
  Apps,
  Databases,
  Desktops,
  Kubes,
  Nodes,
  Roles,
  UserGroups,
} from './ResourceList.story';

test('render Apps', async () => {
  const { container } = render(<Apps />);
  expect(container).toMatchSnapshot();
});

test('render Databases', async () => {
  const { container } = render(<Databases />);
  expect(container).toMatchSnapshot();
});

test('render Desktops', async () => {
  const { container } = render(<Desktops />);
  expect(container).toMatchSnapshot();
});

test('render Kubes', async () => {
  const { container } = render(<Kubes />);
  expect(container).toMatchSnapshot();
});

test('render Nodes', async () => {
  const { container } = render(<Nodes />);
  expect(container).toMatchSnapshot();
});

test('render Roles', async () => {
  const { container } = render(<Roles />);
  expect(container).toMatchSnapshot();
});

test('render UserGroups', async () => {
  const { container } = render(<UserGroups />);
  expect(container).toMatchSnapshot();
});
