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

import { renderHook } from '@testing-library/react';

import { usePrevious } from './usePrevious';

test('usePrevious', () => {
  const { result, rerender } = renderHook(props => usePrevious(props.value), {
    initialProps: { value: 'first-value' },
  });

  // Initial value should be undefined.
  expect(result.current).toBeUndefined();

  // After the value changes, the previous
  // value should be the value.
  rerender({ value: 'second-value' });
  expect(result.current).toBe('first-value');

  // Does not update previous value
  // until the value changes again.
  rerender({ value: 'second-value' });
  expect(result.current).toBe('first-value');

  // Change value again.
  rerender({ value: 'third-value' });
  expect(result.current).toBe('second-value');
});
