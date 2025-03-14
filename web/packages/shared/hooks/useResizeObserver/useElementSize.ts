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

import { useState } from 'react';

import { useResizeObserver } from './';

/**
 * useElementSize returns a ref and the size of the element.
 * The size is updated whenever the element is resized.
 */
export const useElementSize = <T extends HTMLElement = HTMLElement>(
  opts: Parameters<typeof useResizeObserver>[1] = {}
) => {
  const [size, setSize] = useState({ width: 0, height: 0 });
  const ref = useResizeObserver<T>(entry => {
    const { width, height } = entry.contentRect;
    setSize({ width, height });
  }, opts);
  return [ref, size] as const;
};
