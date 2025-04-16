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

import { screen } from '@testing-library/react';

import { fireEvent, render } from 'design/utils/testing';

import { MenuIcon, MenuItem } from '.';

test('basic functionality of clicking is respected', () => {
  render(
    <MenuIcon>
      <MenuItem>Edit</MenuItem>
      <MenuItem>Delete</MenuItem>
    </MenuIcon>
  );

  // prop open is set to false as default
  expect(screen.queryByTestId('Modal')).not.toBeInTheDocument();

  // clicking on button opens menu
  fireEvent.click(screen.getByTestId('button'));
  expect(screen.getByTestId('Modal')).toBeInTheDocument();

  // clicking on menu item closes menu
  fireEvent.click(screen.getByText(/edit/i));
  expect(screen.queryByTestId('Modal')).not.toBeInTheDocument();

  // clicking on button opens menu again
  fireEvent.click(screen.getByTestId('button'));
  expect(screen.getByTestId('Modal')).toBeInTheDocument();

  // clicking on backdrop closes menu
  fireEvent.click(screen.getByTestId('backdrop'));
  expect(screen.queryByTestId('Modal')).not.toBeInTheDocument();
});

const menuListCss = {
  style: {
    right: '10px',
    position: 'absolute',
    top: '10px',
  },
};

test('menuActionProps is respected', () => {
  render(<MenuIcon buttonIconProps={menuListCss} />);
  expect(screen.getByTestId('button')).toHaveStyle(menuListCss.style);
});
