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

import { Status, StatusKind, StatusVariant } from './Status';

describe('design/Status', () => {
  const kinds: StatusKind[] = [
    'success',
    'warning',
    'info',
    'danger',
    'neutral',
    'primary',
  ];

  const variants: StatusVariant[] = ['filled', 'filled-tonal', 'border'];

  it.each(kinds)('renders kind="%s"', kind => {
    render(<Status kind={kind}>Label</Status>);
    expect(screen.getByText('Label')).toBeInTheDocument();
  });

  it.each(variants)('renders variant="%s"', variant => {
    render(
      <Status kind="success" variant={variant}>
        Label
      </Status>
    );
    expect(screen.getByText('Label')).toBeInTheDocument();
  });

  it('renders a custom icon component', () => {
    const CustomIcon = (props: any) => (
      <span data-testid="custom-icon" {...props} />
    );
    render(
      <Status kind="info" icon={CustomIcon}>
        Info
      </Status>
    );
    expect(screen.getByTestId('custom-icon')).toBeInTheDocument();
  });

  it('hides icon when noIcon is set', () => {
    const CustomIcon = (props: any) => (
      <span data-testid="custom-icon" {...props} />
    );

    render(
      <Status kind="success" noIcon icon={CustomIcon}>
        No Icon
      </Status>
    );

    expect(screen.queryByTestId('custom-icon')).not.toBeInTheDocument();
  });
});
