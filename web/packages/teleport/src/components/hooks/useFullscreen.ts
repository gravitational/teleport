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

import fscreen from 'fscreen';
import { useCallback, useEffect, useState, type RefObject } from 'react';

export function useFullScreen(
  ref: RefObject<HTMLElement>
): [boolean, () => void, () => void] {
  const [active, setActive] = useState<boolean>(false);

  useEffect(() => {
    function handleFullscreenChange() {
      setActive(fscreen.fullscreenElement === ref.current);
    }

    fscreen.addEventListener('fullscreenchange', handleFullscreenChange);

    return () => {
      fscreen.removeEventListener('fullscreenchange', handleFullscreenChange);
    };
  }, [ref]);

  const enter = useCallback(() => {
    if (fscreen.fullscreenElement) {
      fscreen.exitFullscreen();

      return fscreen.requestFullscreen(ref.current);
    }

    if (ref.current) {
      return fscreen.requestFullscreen(ref.current);
    }
  }, [ref]);

  const exit = useCallback(() => {
    if (fscreen.fullscreenElement === ref.current) {
      return fscreen.exitFullscreen();
    }
    return Promise.resolve();
  }, [ref]);

  return [active, enter, exit];
}
