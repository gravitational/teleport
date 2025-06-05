/**
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

import { DependencyList, MutableRefObject, useEffect, useRef } from 'react';

/**
 * Returns `ref` object that is automatically focused when `shouldFocus` is `true`.
 * Focus can be also re triggered by changing any of the `refocusDeps`.
 */
export function useRefAutoFocus<T extends { focus(): void }>(options: {
  shouldFocus: boolean;
  /**
   * @deprecated Include items from refocusDeps into the calculation of shouldFocus instead.
   * The list of useEffect deps should be statically known.
   */
  refocusDeps?: DependencyList;
}): MutableRefObject<T> {
  const ref = useRef<T>(undefined);

  useEffect(() => {
    if (options.shouldFocus) {
      ref.current?.focus();
    }
  }, [options.shouldFocus, ref, ...(options.refocusDeps || [])]);

  return ref;
}
