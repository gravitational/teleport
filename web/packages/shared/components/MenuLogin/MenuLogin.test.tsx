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

import { fireEvent, render, screen, waitFor } from 'design/utils/testing';

import { MenuLogin } from './MenuLogin';
import { MenuInputType } from './types';

test('filters options and selects first item when inputType is FILTER', async () => {
  const loginItems = [
    { url: '', login: 'user1' },
    { url: '', login: 'user2' },
    { url: '', login: 'admin' },
  ];
  const onSelect = jest.fn();

  render(
    <MenuLogin
      required={false}
      getLoginItems={() => loginItems}
      onSelect={onSelect}
      inputType={MenuInputType.FILTER}
    />
  );

  fireEvent.click(await screen.findByText(/connect/i));

  // Type 'user' into the input to filter
  const input = await screen.findByPlaceholderText('Search logins…');
  fireEvent.change(input, { target: { value: 'user' } });

  fireEvent.keyPress(input, {
    key: 'Enter',
    keyCode: 13,
  });

  await waitFor(() => {
    expect(onSelect).toHaveBeenCalledWith(expect.anything(), 'user1');
  });

  // Verify that 'admin' is not visible in the filtered list
  expect(screen.queryByText('admin')).not.toBeInTheDocument();
});

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
