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

import { render } from 'design/utils/testing';

import { IconExample } from './Menu.story';

describe('design/Menu', () => {
  it('renders parent and its children', () => {
    render(<IconExample />);

    const parent = screen.getByTestId('Modal');
    const menu = screen.getByRole('menu');
    const item = screen.getAllByTestId('item');
    const icon = screen.getAllByTestId('icon');

    expect(parent).toBeInTheDocument();
    expect(menu).toBeInTheDocument();
    expect(item).toHaveLength(3);
    expect(icon).toHaveLength(3);
  });
});
