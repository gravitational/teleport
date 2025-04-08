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

import { WithMultipleRows, WithSingleRow } from './MultiRowBox.story';

test('renders single row', () => {
  const { container } = render(<WithSingleRow />);
  expect(container.firstChild).toMatchSnapshot();
});

test('renders multiple rows', () => {
  const { container } = render(<WithMultipleRows />);
  expect(container.firstChild).toMatchSnapshot();
});
