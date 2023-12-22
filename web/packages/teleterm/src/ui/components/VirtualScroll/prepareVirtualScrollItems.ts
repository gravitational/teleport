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

import { VirtualScrollProps } from './types';

interface VirtualScrollItem<T> {
  item: T;
  depth: number;
  isLeaf: boolean;
}

export function prepareVirtualScrollItems<T>(
  options: Pick<VirtualScrollProps<T>, 'items' | 'keyProp' | 'childrenProp'> & {
    expandedKeys: Set<unknown>;
  }
) {
  function getFlattenedItems(items: T[], depth = 0): VirtualScrollItem<T>[] {
    return items.reduce<VirtualScrollItem<T>[]>((flattenedItems, item) => {
      const hasChildren = item =>
        Array.isArray(item[options.childrenProp]) &&
        item[options.childrenProp]?.length;
      const isLeaf = !hasChildren(item);
      const virtualScrollItem = { item, depth, isLeaf };

      if (isLeaf || !options.expandedKeys.has(item[options.keyProp])) {
        return [...flattenedItems, virtualScrollItem];
      }

      return [
        ...flattenedItems,
        virtualScrollItem,
        ...getFlattenedItems(
          item[options.childrenProp] as unknown as T[],
          depth + 1
        ),
      ];
    }, []);
  }

  return getFlattenedItems(options.items);
}
