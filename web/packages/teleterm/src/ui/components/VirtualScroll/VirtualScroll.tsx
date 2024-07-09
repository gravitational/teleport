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

import React, { useRef, useEffect } from 'react';
import styled from 'styled-components';

import { useVirtualScrollNodes } from './useVirtualScrollNodes';
import { VirtualScrollProps } from './types';

export function VirtualScroll<T>(props: VirtualScrollProps<T>) {
  // consider using `content-visibility: auto` https://github.com/gravitational/webapps/pull/595#pullrequestreview-880424544

  const scrollableRef = useRef<HTMLDivElement>();
  const {
    setScrollTop,
    updateRenderedNodesCount,
    visibleNodes,
    offset,
    totalHeight,
  } = useVirtualScrollNodes(props);

  function handleScroll(e: React.UIEvent<HTMLDivElement>): void {
    setScrollTop(e.currentTarget.scrollTop);
  }

  useEffect(() => {
    const resizeObserver = new ResizeObserver(entries => {
      updateRenderedNodesCount(entries[0].contentRect.height);
    });

    resizeObserver.observe(scrollableRef.current);

    return () => {
      resizeObserver.unobserve(scrollableRef.current);
      updateRenderedNodesCount.cancel();
    };
  }, []);

  return (
    <Scrollable ref={scrollableRef} onScroll={handleScroll}>
      <TotalHeight height={totalHeight}>
        <Offset moveBy={offset}>{visibleNodes}</Offset>
      </TotalHeight>
    </Scrollable>
  );
}

const TotalHeight = styled.div<{ height: number }>`
  height: ${props => props.height + 'px'};
`;

const Offset = styled.div.attrs((props: { moveBy: number }) => ({
  style: {
    transform: `translateY(${props.moveBy + 'px'})`,
  },
}))<{ moveBy: number }>``;

const Scrollable = styled.div`
  height: 100%;
  overflow-y: auto;
`;
