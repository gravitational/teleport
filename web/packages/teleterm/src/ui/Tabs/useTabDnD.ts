/*
Copyright 2021 Gravitational, Inc.

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

import { MutableRefObject } from 'react';
import { useDrag, useDrop } from 'react-dnd';

const TAB_ITEM_TYPE = 'TAB_ITEM_TYPE';

export function useTabDnD({ index, onDrop, ref, canDrag }: Props): {
  isDragging: boolean;
} {
  const [{ isDragging }, drag] = useDrag({
    type: TAB_ITEM_TYPE,
    item: () => {
      return { index };
    },
    collect: monitor => ({
      isDragging: monitor.isDragging(),
    }),
    canDrag,
  });

  const [, drop] = useDrop({
    accept: TAB_ITEM_TYPE,
    hover(item: Pick<Props, 'index'>, monitor) {
      const dragIndex = item.index;
      const hoverIndex = index;

      // Don't replace items with themselves
      if (dragIndex === hoverIndex) {
        return;
      }

      const hoverBoundingRect = ref.current?.getBoundingClientRect();
      const hoverMiddleX = hoverBoundingRect.width / 2;

      // Determine mouse position
      const clientOffset = monitor.getClientOffset();

      // Get pixels to the left
      const hoverClientX = clientOffset.x - hoverBoundingRect.left;

      // Only perform the move when the mouse has crossed half of the item width
      if (dragIndex < hoverIndex && hoverClientX < hoverMiddleX) {
        return;
      }
      if (dragIndex > hoverIndex && hoverClientX > hoverMiddleX) {
        return;
      }

      onDrop(dragIndex, hoverIndex);
      item.index = hoverIndex;
    },
  });

  drag(drop(ref));

  return { isDragging };
}

interface Props {
  index: number;

  onDrop(oldIndex: number, newIndex: number): void;

  ref?: MutableRefObject<HTMLElement>;
  canDrag?: boolean;
}
