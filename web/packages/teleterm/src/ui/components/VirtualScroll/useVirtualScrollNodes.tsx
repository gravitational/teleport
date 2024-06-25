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

import React, { Fragment, useMemo, useState } from 'react';
import { debounce } from 'shared/utils/highbar';
import styled from 'styled-components';

import { prepareVirtualScrollItems } from './prepareVirtualScrollItems';
import { VirtualScrollProps } from './types';

export function useVirtualScrollNodes<T>(props: VirtualScrollProps<T>) {
  const renderedNodesMultiplier = 2;
  const [expandedKeys, setExpandedKeys] = useState(new Set<unknown>());
  const [renderedNodesLength, setRenderedNodesLength] = useState<number>();
  const [scrollTop, setScrollTop] = useState(0);

  const items = useMemo(
    () =>
      prepareVirtualScrollItems<T>({
        expandedKeys,
        items: props.items,
        childrenProp: props.childrenProp,
        keyProp: props.keyProp,
      }),
    [expandedKeys, props.items, props.childrenProp, props.keyProp]
  );

  // these items are needed to be pre-rendered above visible items
  const topPrereneredItems = useMemo(() => {
    function getTopItemsCount() {
      const value = Math.floor(
        renderedNodesLength / renderedNodesMultiplier / 2
      );
      if (Number.isNaN(value)) {
        return 0;
      }
      return value;
    }

    return Array.from(new Array(getTopItemsCount()))
      .fill(0)
      .map(() => undefined);
  }, [renderedNodesLength, renderedNodesMultiplier]);

  const firstIndexToRender = Math.floor(scrollTop / props.rowHeight);

  const visibleNodes = [...topPrereneredItems, ...items]
    .slice(firstIndexToRender, firstIndexToRender + renderedNodesLength)
    .map((i, index) => {
      if (i === undefined) {
        return (
          <EmptyNode key={`_empty_node_${index}`} height={props.rowHeight} />
        );
      }

      const itemKey = i.item[props.keyProp];
      return (
        <Fragment key={itemKey.toString()}>
          {props.Node({
            item: i.item,
            isExpanded: expandedKeys.has(itemKey),
            isLeaf: i.isLeaf,
            depth: i.depth,
            onToggle: () => {
              setExpandedKeys(prevExpandedKeys => {
                const newExpandedKeys = new Set(prevExpandedKeys);
                if (!newExpandedKeys.has(itemKey)) {
                  newExpandedKeys.add(itemKey);
                } else {
                  newExpandedKeys.delete(itemKey);
                }
                return newExpandedKeys;
              });
            },
          })}
        </Fragment>
      );
    });

  const totalHeight = props.rowHeight * items.length;
  const offset =
    (firstIndexToRender - topPrereneredItems.length) * props.rowHeight;

  const updateRenderedNodesCount = useMemo(
    () =>
      debounce((viewportHeight: number) => {
        const visibleItemsLength = Math.ceil(
          (viewportHeight / props.rowHeight) * renderedNodesMultiplier
        );
        setRenderedNodesLength(visibleItemsLength);
      }, 10),
    [setRenderedNodesLength, renderedNodesMultiplier, props.rowHeight]
  );

  return {
    totalHeight,
    offset,
    visibleNodes,
    setScrollTop,
    updateRenderedNodesCount,
  };
}

const EmptyNode = styled.div<{ height: number }>`
  width: 100%;
  height: ${props => props.height + 'px'};
`;
