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

import { render, theme } from 'design/utils/testing';

import Label, { Danger, Primary, Secondary, Warning } from './Label';

describe('design/Label', () => {
  const colors = [
    theme.colors.brand,
    theme.colors.spotBackground[0],
    theme.colors.warning.main,
    theme.colors.error.main,
  ];

  test.each`
    kind                  | Component    | expected
    ${'default: primary'} | ${Label}     | ${colors[0]}
    ${'primary'}          | ${Primary}   | ${colors[0]}
    ${'secondary'}        | ${Secondary} | ${colors[1]}
    ${'warning'}          | ${Warning}   | ${colors[2]}
    ${'danger'}           | ${Danger}    | ${colors[3]}
  `('component renders $kind label', ({ Component, expected }) => {
    const { container } = render(<Component />);
    expect(container.firstChild).toHaveStyle({
      'background-color': expected,
    });
  });
});
