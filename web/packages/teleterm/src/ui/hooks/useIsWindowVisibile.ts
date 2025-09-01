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

import { useEffect, useState } from 'react';

import { useAppContext } from 'teleterm/ui/appContextProvider';

/** Returns whether the window is visible. */
export function useIsWindowVisibile() {
  const ctx = useAppContext();
  // We assume that the window is visible when the app starts.
  // This may change in the future.
  const [visible, setVisible] = useState(true);

  useEffect(() => {
    const { cleanup } = ctx.mainProcessClient.subscribeToWindowVisibility(
      ({ visible }) => {
        setVisible(visible);
      }
    );

    return cleanup;
  }, [ctx.mainProcessClient]);

  return visible;
}
