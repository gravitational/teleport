/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { screen } from '@testing-library/react';

import { render, fireEvent } from 'design/utils/testing';

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
