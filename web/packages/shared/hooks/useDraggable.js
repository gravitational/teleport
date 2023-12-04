/*
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

import { useState, useCallback, useEffect } from 'react';

const POSITION = { x: 0, y: 0 };

export default function useDraggable() {
  const [state, setState] = useState({
    isDragging: false,
    origin: POSITION,
    position: POSITION,
  });

  const onMouseDown = useCallback(event => {
    // disable text selection
    event.stopPropagation();
    event.preventDefault();
    const { clientX, clientY } = event;
    setState(state => ({
      ...state,
      isDragging: true,
      origin: { x: clientX, y: clientY },
    }));
  }, []);

  const onMouseMove = useCallback(
    event => {
      const position = {
        x: event.clientX - state.origin.x,
        y: event.clientY - state.origin.y,
      };

      setState(state => ({
        ...state,
        position,
      }));
    },
    [state.origin]
  );

  const onMouseUp = useCallback(() => {
    setState(state => ({
      ...state,
      isDragging: false,
    }));
  }, []);

  useEffect(() => {
    if (state.isDragging) {
      window.addEventListener('mousemove', onMouseMove);
      window.addEventListener('mouseup', onMouseUp);
    } else {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);

      setState(state => ({ ...state, position: { x: 0, y: 0 } }));
    }
  }, [state.isDragging, onMouseMove, onMouseUp]);

  return {
    onMouseDown,
    isDragging: state.isDragging,
    position: state.position,
  };
}
