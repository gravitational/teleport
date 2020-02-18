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
import ActionMenu, { MenuItem } from '.';
import { render, fireEvent } from 'design/utils/testing';

test('basic functionality of clicking is respected', () => {
  const { queryByTestId, getByTestId, getByText } = render(
    <ActionMenu>
      <MenuItem>Edit</MenuItem>
      <MenuItem>Delete</MenuItem>
    </ActionMenu>
  );

  // prop open is set to false as default
  expect(queryByTestId('Modal')).not.toBeInTheDocument();

  // clicking on button opens menu
  fireEvent.click(getByTestId('button'));
  expect(getByTestId('Modal')).toBeInTheDocument();

  // clicking on menu item closes menu
  fireEvent.click(getByText(/edit/i));
  expect(queryByTestId('Modal')).not.toBeInTheDocument();

  // clicking on button opens menu again
  fireEvent.click(getByTestId('button'));
  expect(getByTestId('Modal')).toBeInTheDocument();

  // clicking on backdrop closes menu
  fireEvent.click(getByTestId('backdrop'));
  expect(queryByTestId('Modal')).not.toBeInTheDocument();
});

const menuListCss = {
  style: {
    right: '10px',
    position: 'absolute',
    top: '10px',
  },
};
test('menuActionProps is respected', () => {
  const { getByTestId } = render(<ActionMenu buttonIconProps={menuListCss} />);

  expect(getByTestId('button')).toHaveStyle(menuListCss.style);
});
