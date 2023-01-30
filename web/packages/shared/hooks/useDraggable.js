/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
