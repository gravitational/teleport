/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { DependencyList, MutableRefObject, useEffect, useRef } from 'react';

/**
 * Returns `ref` object that is automatically focused when `shouldFocus` is `true`.
 * Focus can be also re triggered by changing any of the `refocusDeps`.
 */
export function useRefAutoFocus<T extends { focus(): void }>(options: {
  shouldFocus: boolean;
  refocusDeps?: DependencyList;
}): MutableRefObject<T> {
  const ref = useRef<T>();

  useEffect(() => {
    if (options.shouldFocus) {
      ref.current?.focus();
    }
  }, [options.shouldFocus, ref, ...(options.refocusDeps || [])]);

  return ref;
}
