/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
    } catch (err) {
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
