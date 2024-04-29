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
  LoadedSearchPending,
  LoadedRolePending,
  LoadedRoleApproved,
  LoadedRoleDenied,
} from './RequestView.story';

test('loaded pending role based request state', () => {
  const { container } = render(<LoadedRolePending />);
  expect(container).toMatchSnapshot();
});

test('loaded pending search based request state', () => {
  const { container } = render(<LoadedSearchPending />);
  expect(container).toMatchSnapshot();
});

test('loaded approved role based request state', () => {
  const { container } = render(<LoadedRoleApproved />);
  expect(container).toMatchSnapshot();
});

test('loaded denied role based request state', () => {
  const { container } = render(<LoadedRoleDenied />);
  expect(container).toMatchSnapshot();
});
