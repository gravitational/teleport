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

import {
  Dispatch,
  SetStateAction,
  useCallback,
  useEffect,
  useState,
} from 'react';

export function useLocalStorage<T>(
  key: string,
  initialValue: T
): [T, Dispatch<SetStateAction<T>>] {
  const read = useCallback(() => {
    const value = window.localStorage.getItem(key);

    if (!value) {
      return initialValue;
    }

    try {
      return JSON.parse(value) as T;
    } catch {
      return initialValue;
    }
  }, [initialValue, key]);

  const [storedValue, setStoredValue] = useState<T>(read());

  const write = useCallback((value: SetStateAction<T>) => {
    const newValue = value instanceof Function ? value(storedValue) : value;

    window.localStorage.setItem(key, JSON.stringify(newValue));

    setStoredValue(newValue);
  }, []);

  useEffect(() => {
    setStoredValue(read());
  }, []);

  return [storedValue, write];
}
