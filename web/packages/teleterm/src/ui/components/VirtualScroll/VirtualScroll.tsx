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

const TotalHeight = styled.div`
  height: ${props => props.height + 'px'};
`;

const Offset = styled.div.attrs(props => ({
  style: {
    transform: `translateY(${props.moveBy + 'px'})`,
  },
}))``;

const Scrollable = styled.div`
  height: 100%;
  overflow-y: auto;
`;
