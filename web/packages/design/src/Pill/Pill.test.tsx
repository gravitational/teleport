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

import { Pill } from './Pill';

describe('design/Pill', () => {
  it('renders the label without dismissable', () => {
    render(<Pill label="arch: x86_64" />);

    expect(screen.getByText('arch: x86_64')).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('render the label with dismissable', () => {
    render(<Pill label="arch: x86_64" onDismiss={() => {}} />);

    expect(screen.getByText('arch: x86_64')).toBeInTheDocument();
    expect(screen.getByRole('button')).toBeVisible();
  });

  it('dismissing pill calls onDismiss', () => {
    const cb = jest.fn();
    render(<Pill label="arch: x86_64" onDismiss={cb} />);

    fireEvent.click(screen.getByRole('button'));

    expect(cb).toHaveBeenCalledWith('arch: x86_64');
  });
});
