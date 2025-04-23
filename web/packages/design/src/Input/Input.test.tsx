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

import { render, screen, theme } from 'design/utils/testing';

import Input from './Input';

describe('design/Input', () => {
  it('forwards a ref', () => {
    const ref = jest.fn();
    render(<Input ref={ref} defaultValue="foo" />);
    expect(ref).toHaveBeenCalledWith(expect.objectContaining({ value: 'foo' }));
  });
  it('respects hasError prop', () => {
    render(<Input hasError={true} />);
    expect(screen.getByRole('textbox')).toHaveStyle({
      'border-color': theme.colors.interactive.solid.danger.default,
    });
    expect(screen.getByRole('graphics-symbol')).toHaveAttribute(
      'aria-label',
      'Error'
    );
  });
});
