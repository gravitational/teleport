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
import { render, fireEvent, screen, waitFor } from 'design/utils/testing';

import { MenuLogin } from './MenuLogin';

test('does not accept an empty value when required is set to true', async () => {
  const onSelect = jest.fn();
  render(
    <MenuLogin
      placeholder="MenuLogin input"
      required={true}
      getLoginItems={() => []}
      onSelect={() => onSelect()}
    />
  );

  fireEvent.click(await screen.findByText(/connect/i));
  fireEvent.keyPress(await screen.findByPlaceholderText('MenuLogin input'), {
    key: 'Enter',
    keyCode: 13,
  });

  expect(onSelect).toHaveBeenCalledTimes(0);
});

test('accepts an empty value when required is set to false', async () => {
  const onSelect = jest.fn();
  render(
    <MenuLogin
      placeholder="MenuLogin input"
      required={false}
      getLoginItems={() => []}
      onSelect={() => onSelect()}
    />
  );

  fireEvent.click(await screen.findByText(/connect/i));
  fireEvent.keyPress(await screen.findByPlaceholderText('MenuLogin input'), {
    key: 'Enter',
    keyCode: 13,
  });

  await waitFor(() => {
    expect(onSelect).toHaveBeenCalledTimes(1);
  });
});
