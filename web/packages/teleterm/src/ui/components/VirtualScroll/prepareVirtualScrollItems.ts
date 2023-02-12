/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
