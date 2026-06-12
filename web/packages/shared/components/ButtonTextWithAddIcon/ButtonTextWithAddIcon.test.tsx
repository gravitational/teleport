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

import { fireEvent, render, screen } from 'design/utils/testing';

import { ButtonTextWithAddIcon } from './ButtonTextWithAddIcon';

test('buttonTextWithAddIcon', () => {
  const onClick = jest.fn();
  const label = 'Add Item';

  const { rerender } = render(
    <ButtonTextWithAddIcon label={label} onClick={() => onClick('click')} />
  );

  expect(screen.getByText('Add Item')).toBeInTheDocument();
  fireEvent.click(screen.getByText('Add Item'));

  expect(onClick).toHaveBeenCalledWith('click');

  rerender(
    <ButtonTextWithAddIcon
      label={label}
      onClick={() => onClick('click')}
      disabled={true}
    />
  );
  expect(screen.getByText('Add Item')).toBeDisabled();
});
