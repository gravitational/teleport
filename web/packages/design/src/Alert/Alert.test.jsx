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

import Alert, { Danger, Info, Warning, Success } from './index';

describe('design/Alert', () => {
  it('respects default "kind" prop == danger', () => {
    const { container } = render(<Alert />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.error.main,
    });
  });

  test('"kind" danger renders bg == theme.colors.error.main', () => {
    const { container } = render(<Danger />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.error.main,
    });
  });

  test('"kind" warning renders bg == theme.colors.warning.main', () => {
    const { container } = render(<Warning />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.warning.main,
    });
  });

  test('"kind" info renders bg == theme.colors.info', () => {
    const { container } = render(<Info />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.info,
    });
  });

  test('"kind" success renders bg == theme.colors.success', () => {
    const { container } = render(<Success />);
    expect(container.firstChild).toHaveStyle({
      background: theme.colors.success.main,
    });
  });
});
