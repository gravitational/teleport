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

import { render, screen } from 'design/utils/testing';

import * as stories from './UserDelete.story';

test('processing state', () => {
  render(<stories.Processing />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('failed state', () => {
  render(<stories.Failed />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('confirm state', () => {
  render(<stories.Confirm />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});
