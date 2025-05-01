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

import { MenuItem } from 'design';
import { fireEvent, render, screen } from 'design/utils/testing';
import { MenuInputType } from 'shared/components/MenuLogin';

import { MenuLoginWithActionMenu } from './MenuLoginWithActionMenu';

test('clicking on primary menu and secondary menu opens respective menu items', async () => {
  render(
    <MenuLoginWithActionMenu
      buttonText="Click Me"
      getLoginItems={() => [{ url: '', login: 'alice' }]}
      onSelect={() => null}
    >
      <MenuItem>Menu item</MenuItem>
    </MenuLoginWithActionMenu>
  );

  fireEvent.click(screen.getByText('Click Me'));
  expect(await screen.findByText('alice')).toBeInTheDocument();

  fireEvent.click(screen.getByTitle('Open menu'));
  expect(await screen.findByText('Menu item')).toBeInTheDocument();
});

test('filter input field should be visible by default', async () => {
  render(
    <MenuLoginWithActionMenu
      buttonText="Click Me"
      getLoginItems={() => [
        { url: '', login: 'alice' },
        { url: '', login: 'bob' },
      ]}
      onSelect={() => null}
      placeholder="search me"
    >
      <MenuItem>Menu item</MenuItem>
    </MenuLoginWithActionMenu>
  );

  fireEvent.click(screen.getByText('Click Me'));
  expect(await screen.findByText('alice')).toBeInTheDocument();

  expect(screen.getByPlaceholderText('search me')).toBeInTheDocument();
});

test('MenuInputType.NONE should show static menu item label', async () => {
  render(
    <MenuLoginWithActionMenu
      buttonText="Click Me"
      getLoginItems={() => [
        { url: '', login: 'alice' },
        { url: '', login: 'bob' },
      ]}
      onSelect={() => null}
      placeholder="search me"
      inputType={MenuInputType.NONE}
    >
      <MenuItem>Menu item</MenuItem>
    </MenuLoginWithActionMenu>
  );

  fireEvent.click(screen.getByText('Click Me'));
  expect(await screen.findByText('alice')).toBeInTheDocument();

  // Verify that placeholder is used as a text label above menu and
  // not as an actual placeholder.
  expect(screen.getByText('search me')).toBeInTheDocument();
  expect(screen.queryByPlaceholderText('search me')).not.toBeInTheDocument();
});
