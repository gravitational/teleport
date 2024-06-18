/*
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

import React from 'react';

import { render, theme } from 'design/utils/testing';

import { Button, ButtonPrimary, ButtonSecondary, ButtonWarning } from './index';

describe('design/Button', () => {
  it('renders a <button> and respects default "kind" prop == primary', () => {
    const { container } = render(<Button />);
    expect(container.firstChild.nodeName).toBe('BUTTON');
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.brand,
    });
  });

  test('"kind" primary renders bg == theme.colors.buttons.primary.default', () => {
    const { container } = render(<ButtonPrimary />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.buttons.primary.default,
    });
  });

  test('"kind" secondary renders bg == theme.colors.buttons.secondary.default', () => {
    const { container } = render(<ButtonSecondary />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.buttons.secondary.default,
    });
  });

  test('"kind" warning renders bg == theme.colors.buttons.warning.default', () => {
    const { container } = render(<ButtonWarning />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.buttons.warning.default,
    });
  });

  test('"size" small renders min-height: 24px', () => {
    const { container } = render(<Button size="small" />);
    expect(container.firstChild).toHaveStyle({ 'min-height': '24px' });
  });

  test('"size" medium renders min-height: 32px', () => {
    const { container } = render(<Button size="medium" />);
    expect(container.firstChild).toHaveStyle('min-height: 32px');
  });

  test('"size" large renders min-height: 40px', () => {
    const { container } = render(<Button size="large" />);
    expect(container.firstChild).toHaveStyle('min-height: 40px');
  });

  test('"block" prop renders width 100%', () => {
    const { container } = render(<Button block />);
    expect(container.firstChild).toHaveStyle('width: 100%');
  });
});
