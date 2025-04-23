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

import { Sample, Tooltip } from './Popover.story';

test('onClick popovers renders', () => {
  render(<Sample />);

  fireEvent.click(screen.getByText('Left'));
  expect(screen.getByTestId('content')).toBeInTheDocument();
  fireEvent.click(screen.getByTestId('backdrop'));
  expect(screen.queryByTestId('content')).not.toBeInTheDocument();
});

test('onMouse tooltip render', () => {
  render(<Tooltip />);

  fireEvent.mouseOver(screen.getByTestId('text'));
  expect(screen.getByTestId('content')).toBeInTheDocument();

  fireEvent.mouseOut(screen.getByTestId('text'));
  expect(screen.queryByTestId('content')).not.toBeInTheDocument();
});
