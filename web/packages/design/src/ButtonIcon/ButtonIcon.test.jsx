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

import { render } from 'design/utils/testing';

import ButtonIcon from './index';

describe('design/ButtonIcon', () => {
  it('renders a <button> and respects default "size" to 1', () => {
    const { container } = render(<ButtonIcon />);
    expect(container.firstChild.nodeName).toBe('BUTTON');
    expect(container.firstChild).toHaveStyle('font-size: 16px');
  });

  test('"size" 0 maps to font-size 12px', () => {
    const { container } = render(<ButtonIcon size={0} />);
    expect(container.firstChild).toHaveStyle('font-size: 12px');
  });

  test('"size" 1 maps to font-size 16px', () => {
    const { container } = render(<ButtonIcon size={1} />);
    expect(container.firstChild).toHaveStyle('font-size: 16px');
  });

  test('"size" 2 maps to font-size 24px', () => {
    const { container } = render(<ButtonIcon size={2} />);
    expect(container.firstChild).toHaveStyle('font-size: 24px');
  });
});
