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

import {
  Dispatch,
  RefObject,
  SetStateAction,
  useCallback,
  useRef,
  useState,
} from 'react';

/**
 * useStateRef creates a piece of state and a ref reflecting a value of the state and a setter that
 * updates both the state and the ref. Useful is situations where an event handler needs to read the
 * value of the state without making its identity dependent on the value of the state.
 *
 * @example
 *
 * const [isOpen, isOpenRef, setIsOpen] = useStateRef(false)
 *
 * const sendNotification = useCallback(() => {
 *   if (isOpenRef.current) {
 *     return;
 *   }
 *
 *   client.sendNotification(foo);
 * }, [client, foo]);
 *
 * useEffect(() => {
 *   setInterval(sendNotification, 5000);
 *
 *   return () => {
 *     // cleanup
 *   }
 * }, [sendNotification])
 */
export function useStateRef<T>(
  initialState: T
): [T, RefObject<T | null>, Dispatch<SetStateAction<T>>] {
  const stateRef = useRef(initialState);
  const [state, setState] = useState(initialState);

  const setStateAndRef = useCallback((newState: SetStateAction<T>) => {
    setState(newState);

    if (typeof newState === 'function') {
      stateRef.current = (newState as (prevState: T) => T)(stateRef.current);
    } else {
      stateRef.current = newState;
    }
  }, []);

  return [state, stateRef, setStateAndRef];
}
