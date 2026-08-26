/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Tag } from './Tag';

describe('design/Tag', () => {
  it('does not render dismiss button by default', () => {
    render(<Tag>tag</Tag>);
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('renders dismiss button when onDismiss is provided', () => {
    render(<Tag onDismiss={() => {}}>tag</Tag>);
    expect(screen.getByRole('button', { name: 'Remove' })).toBeInTheDocument();
  });

  it('calls onDismiss when dismiss button is clicked', () => {
    const onDismiss = jest.fn();
    render(<Tag onDismiss={onDismiss}>tag</Tag>);

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }));

    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it('dismiss click does not propagate to parent onClick', () => {
    const onClick = jest.fn();
    const onDismiss = jest.fn();
    render(
      <Tag onClick={onClick} onDismiss={onDismiss}>
        tag
      </Tag>
    );

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }));

    expect(onDismiss).toHaveBeenCalledTimes(1);
    expect(onClick).not.toHaveBeenCalled();
  });
});
