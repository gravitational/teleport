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
  createContext,
  PropsWithChildren,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';

interface LayoutContextValue {
  hasDockedElement: boolean;
  setHasDockedElement: (value: boolean) => void;
  currentWidth: number;
}

const LayoutContext = createContext<LayoutContextValue>(null);

export function LayoutContextProvider(props: PropsWithChildren<unknown>) {
  const [hasDockedElement, setHasDockedElement] = useState(false);
  const [currentWidth, setCurrentWidth] = useState(window.innerWidth);
  const containerRef = useRef<HTMLDivElement>();

  useEffect(() => {
    // TODO(ravicious): Use useResizeObserver instead. Ensure that the callback passed to
    // useResizeObserver has a stable identity.
    const resizeObserver = new ResizeObserver(entries => {
      const container = entries[0];
      setCurrentWidth(container?.contentRect.width || 0);
    });

    resizeObserver.observe(containerRef.current);
    return () => {
      resizeObserver.disconnect();
    };
  }, []);

  return (
    <LayoutContext.Provider
      value={{ hasDockedElement, setHasDockedElement, currentWidth }}
    >
      <div ref={containerRef}>{props.children}</div>
    </LayoutContext.Provider>
  );
}

export function useLayout() {
  return useContext(LayoutContext);
}
