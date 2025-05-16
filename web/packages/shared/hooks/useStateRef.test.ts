/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { act, renderHook } from '@testing-library/react';

import { useStateRef } from './useStateRef';

it('updates both state and ref at the same time when passing a regular value', () => {
  const { result } = renderHook(props => useStateRef(props.initialState), {
    initialProps: { initialState: false },
  });
  let [state, ref, setState] = result.current;
  expect(state).toBe(false);
  expect(ref.current).toBe(false);

  act(() => {
    setState(true);
  });

  [state, ref] = result.current;
  expect(state).toBe(true);
  expect(ref.current).toBe(true);

  // @ts-expect-error ref.current should be readonly.
  ref.current = false;
});

it('updates both state and ref at the same time when passing an updater function', () => {
  const { result } = renderHook(props => useStateRef(props.initialState), {
    initialProps: { initialState: false },
  });
  let [state, ref, setState] = result.current;
  expect(state).toBe(false);
  expect(ref.current).toBe(false);

  act(() => {
    setState(currentValue => !currentValue);
  });

  [state, ref] = result.current;
  expect(state).toBe(true);
  expect(ref.current).toBe(true);
});
