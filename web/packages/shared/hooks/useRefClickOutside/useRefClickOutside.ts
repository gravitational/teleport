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

import { MutableRefObject, useEffect, useRef, useCallback } from 'react';

// IGNORE_CLICK_CLASSNAME is the className that should be on elements which shouldn't trigger setOpen(false).
export const IGNORE_CLICK_CLASSNAME = 'ignore-click';

/**
 * useRefClickOutside adds a `mousedown` event listener (and cleanup)
 * that upon `mousedown` will:
 *   - set the field `open` to false if the event happened outside the ref
 *   - does nothing if the event happened inside the ref
 *
 * Returns a `ref` object to be used for the element that we want `mousedown`
 * events to be ignored.
 */
export function useRefClickOutside<
  T extends { contains(eventTarget: HTMLElement): boolean },
>(options: { open: boolean; setOpen(b: boolean): void }): MutableRefObject<T> {
  const ref = useRef<T>();
  const { setOpen, open } = options;

  const handleClickOutside = useCallback(
    (event: MouseEvent) => {
      if (
        ref.current &&
        !(
          ref.current.contains(event.target as HTMLElement) ||
          (event.target as HTMLElement).closest('.' + IGNORE_CLICK_CLASSNAME)
        )
      ) {
        setOpen(false);
      }
    },
    [setOpen]
  );

  useEffect(() => {
    if (open) {
      document.addEventListener('mousedown', handleClickOutside);

      return () => {
        document.removeEventListener('mousedown', handleClickOutside);
      };
    }
  }, [open, handleClickOutside]);

  return ref;
}
