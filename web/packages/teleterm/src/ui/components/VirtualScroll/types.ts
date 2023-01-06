import { ReactNode } from 'react';

export interface VirtualScrollProps<T> {
  rowHeight: number;
  keyProp: keyof T;
  childrenProp?: keyof T;
  items: T[];
  Node(props: {
    item: T;
    depth: number;
    isExpanded: boolean;
    isLeaf: boolean;
    onToggle(): void;
  }): ReactNode;
}
