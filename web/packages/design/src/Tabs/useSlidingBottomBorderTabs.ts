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

import { useLayoutEffect, useRef } from 'react';

/**
 * On selecting a new tab, produces a bottom border sliding animation
 * from the current tab to the newly selected tab.
 *
 * Requires that each TabContainer defines a prop called "data-tab-id".
 *
 * TODO: mimics the implementation of tabs in
 * e/web/teleport/src/AccessMonitoring/AccessMonitoring.tsx.
 * Consider updating useSlidingBottomBorderTabs to be plain css.
 */
export function useSlidingBottomBorderTabs<T>({ activeTab }: { activeTab: T }) {
  const borderRef = useRef<HTMLDivElement>(null);
  const parentRef = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    if (!parentRef.current || !borderRef.current) {
      return;
    }

    const activeElement = parentRef.current.querySelector(
      `[data-tab-id="${activeTab}"]`
    );

    if (activeElement) {
      const parentBounds = parentRef.current.getBoundingClientRect();
      const activeBounds = activeElement.getBoundingClientRect();

      const left = activeBounds.left - parentBounds.left;
      const width = activeBounds.width;

      borderRef.current.style.left = `${left}px`;
      borderRef.current.style.width = `${width}px`;
    }
  }, [activeTab]);

  return { borderRef, parentRef };
}
