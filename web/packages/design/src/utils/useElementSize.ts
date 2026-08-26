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

import { useCallback, useState } from 'react';

import { useResizeObserver } from './useResizeObserver';

/**
 * useElementSize returns a ref and the size of the element.
 * The size is updated whenever the element is resized.
 *
 * @example
 * // With default initial size (width = 0, height = 0)
 * const Component = () => {
 *   const [ref, { width, height }] = useElementSize<HTMLDivElement>();
 *
 *   return (
 *     <>
 *       <div ref={ref}>This element's size is being observed</div>
 *       <p>Width: {width}px, Height: {height}px</p>
 *     </>
 *   );
 * };
 *
 * @example
 * // With custom initial size
 * const Component = () => {
 *   const [ref, { width, height }] = useElementSize<HTMLDivElement>({ height: undefined });
 *
 *   return <div ref={ref}>This element starts with an observed height of 'undefined'</div>;
 * };
 */
export const useElementSize = <T extends HTMLElement = HTMLElement>(
  initialSize: { width?: number; height?: number } = { width: 0, height: 0 },
  opts: Parameters<typeof useResizeObserver>[1] = {}
) => {
  const [size, setSize] = useState({
    width: 'width' in initialSize ? initialSize.width : 0,
    height: 'height' in initialSize ? initialSize.height : 0,
  });

  const handleResize = useCallback((entry: ResizeObserverEntry) => {
    const { width, height } = entry.contentRect;
    setSize({ width, height });
  }, []);

  const ref = useResizeObserver<T>(handleResize, opts);

  return [ref, size] as const;
};
