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

import { MenuItem } from 'design';
import { render, screen, userEvent } from 'design/utils/testing';

import { ButtonWithMenu } from './ButtonWithMenu';

test('clicking on a menu item executes its onClick and closes the menu', async () => {
  const user = userEvent.setup();
  const menuItemOnClick = jest.fn();

  render(
    <ButtonWithMenu text="Button text">
      <MenuItem onClick={menuItemOnClick}>Menu item</MenuItem>
    </ButtonWithMenu>
  );

  expect(screen.queryByText('Menu item')).not.toBeInTheDocument();

  await user.click(screen.getByTitle('Open menu'));
  await user.click(screen.getByText('Menu item'));

  // Verify that the menu was closed.
  expect(screen.queryByText('Menu item')).not.toBeInTheDocument();

  expect(menuItemOnClick).toHaveBeenCalledTimes(1);
});

test('individual menu items can stop propagation to keep the menu open after click', async () => {
  const user = userEvent.setup();
  const menuItemOnClick = (e: React.MouseEvent) => e.stopPropagation();

  render(
    <ButtonWithMenu text="Button text">
      <MenuItem onClick={menuItemOnClick}>Menu item</MenuItem>
    </ButtonWithMenu>
  );

  expect(screen.queryByText('Menu item')).not.toBeInTheDocument();

  await user.click(screen.getByTitle('Open menu'));
  await user.click(screen.getByText('Menu item'));

  // Verify that the menu stayed open.
  expect(screen.getByText('Menu item')).toBeInTheDocument();
});
