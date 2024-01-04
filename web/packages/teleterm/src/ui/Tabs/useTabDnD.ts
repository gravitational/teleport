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
