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

import { render } from 'design/utils/testing';

import { Status } from './Status';

describe('design/Status', () => {
  it('renders a component reference icon with color and size', () => {
    const CustomIcon = (props: any) => (
      <span
        data-testid="comp-icon"
        data-color={props.color}
        data-size={props.size}
      />
    );
    render(
      <Status kind="success" icon={CustomIcon}>
        Custom
      </Status>
    );
    const icon = screen.getByTestId('comp-icon');
    expect(icon).toBeInTheDocument();
    expect(icon.dataset.color).toBeTruthy();
    expect(icon.dataset.size).toBeTruthy();
  });
});
