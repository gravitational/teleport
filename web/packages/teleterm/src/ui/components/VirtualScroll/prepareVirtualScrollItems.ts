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
