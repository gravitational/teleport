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

import { useEffect, useMemo, useState } from 'react';

import Store from './store';

// This is the primary method to subscribe to store updates
// using React hooks mechanism.
export default function useStore<T extends Store<any>>(store: T): T {
  const [, rerender] = useState<any>();
  const memoizedState = useMemo(() => store.state, [store.state]);

  useEffect(() => {
    function syncState() {
      // do not re-render if state has not changed since last call
      if (memoizedState !== store.state) {
        rerender({});
      }
    }

    function onChange() {
      syncState();
    }

    // Sync state and force re-render if store has changed
    // during Component mount cycle
    syncState();
    // Subscribe to store changes
    store.subscribe(onChange);

    // Unsubscribe from store
    function cleanup() {
      store.unsubscribe(onChange);
    }

    return cleanup;
  }, [store]);

  return store;
}
