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

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type RefObject,
} from 'react';

export function useFullscreen(ref: RefObject<HTMLElement>) {
  const [active, setActive] = useState<boolean>(false);

  useEffect(() => {
    function handleFullscreenChange() {
      setActive(document.fullscreenElement === ref.current);
    }

    document.addEventListener('fullscreenchange', handleFullscreenChange);

    return () => {
      document.removeEventListener('fullscreenchange', handleFullscreenChange);
    };
  }, [ref]);

  const enter = useCallback(async () => {
    if (document.fullscreenElement) {
      await document.exitFullscreen();
    }

    if (ref.current) {
      return ref.current.requestFullscreen();
    }
  }, [ref]);

  const exit = useCallback(() => {
    if (document.fullscreenElement === ref.current) {
      return document.exitFullscreen();
    }

    return Promise.resolve();
  }, [ref]);

  return useMemo(() => ({ active, enter, exit }), [active, enter, exit]);
}
