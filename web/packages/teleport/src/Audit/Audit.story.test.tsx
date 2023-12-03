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

import { LoadedSample, AllPossibleEvents } from './Audit.story';

test('loaded audit log screen', async () => {
  const { container } = render(<LoadedSample />);
  await screen.findByText(/Audit Log/);
  expect(container.firstChild).toMatchSnapshot();
});

test('list of all events', async () => {
  const { container } = render(<AllPossibleEvents />);
  expect(container).toMatchSnapshot();
});
