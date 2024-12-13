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

import LabelState, {
  StateDanger,
  StateInfo,
  StateWarning,
  StateSuccess,
} from './LabelState';

const colors = {
  primary: theme.colors.brand,
  info: theme.colors.spotBackground[0],
  warning: theme.colors.warning.main,
  danger: theme.colors.error.main,
  success: theme.colors.success.main,
};

describe('design/LabelState', () => {
  test.each`
    Component       | kind           | expected
    ${LabelState}   | ${'primary'}   | ${colors.primary}
    ${StateInfo}    | ${'secondary'} | ${colors.info}
    ${StateWarning} | ${'warning'}   | ${colors.warning}
    ${StateDanger}  | ${'danger'}    | ${colors.danger}
    ${StateSuccess} | ${'success'}   | ${colors.success}
  `('respects kind prop set to $kind', ({ Component, expected }) => {
    const { container } = render(<Component />);
    expect(container.firstChild).toHaveStyle({
      background: expected,
    });

    expect(getComputedStyle(container.firstChild).boxShadow).toBe('');
  });

  it('respects shadow prop', () => {
    const { container } = render(<LabelState shadow={true} />);
    expect(getComputedStyle(container.firstChild).boxShadow).toEqual(
      expect.any(String)
    );
  });
});
