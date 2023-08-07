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

import { useEffect, useState } from 'react';

export const SHOW_HINT_TIMEOUT = 1000 * 60 * 5; // 5 minutes

export function useShowHint(enabled: boolean) {
  const [showHint, setShowHint] = useState(false);

  useEffect(() => {
    if (enabled) {
      const id = window.setTimeout(() => setShowHint(true), SHOW_HINT_TIMEOUT);

      return () => window.clearTimeout(id);
    }
  }, [enabled]);

  return showHint;
}
