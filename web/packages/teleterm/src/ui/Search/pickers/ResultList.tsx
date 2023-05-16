/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, {
  ReactElement,
  useEffect,
  useMemo,
  useRef,
  useState,
  useCallback,
} from 'react';
import { Flex } from 'design';
import styled, { css } from 'styled-components';
import { Attempt } from 'shared/hooks/useAsync';

import LinearProgress from 'teleterm/ui/components/LinearProgress';

import { AddWindowEventListener } from '../SearchContext';

type ResultListProps<T> = {
  /**
   * List of attempts containing results to render.
   * Displayed items will follow the order of attempts.
   * If any attempt is loading, then the loading bar is visible.
   */
  attempts: Attempt<T[]>[];
  /**
   * ExtraTopComponent is the element that is rendered above the items.
   */
  ExtraTopComponent?: ReactElement;
  onPick(item: T): void;
  onBack(): void;
  render(item: T): { Component: ReactElement; key: string };
  addWindowEventListener: AddWindowEventListener;
};

export function ResultList<T>(props: ResultListProps<T>) {
  const {
    attempts,
    ExtraTopComponent,
    onPick,
    onBack,
    addWindowEventListener,
  } = props;
  const activeItemRef = useRef<HTMLDivElement>();
  const [activeItemIndex, setActiveItemIndex] = useState(0);
  const pickAndResetActiveItem = useCallback(
    (item: T) => {
      setActiveItemIndex(0);
      onPick(item);
    },
    [onPick]
  );

  const items = useMemo(() => {
    return attempts.map(a => a.data || []).flat();
  }, [attempts]);

  // Reset the active item index if it's greater than the number of available items.
  // This can happen in cases where the user selects the nth item and then filters the list so that
  // there's only one item.
  if (activeItemIndex !== 0 && activeItemIndex >= items.length) {
    setActiveItemIndex(0);
  }

  useEffect(() => {
    const handleArrowKey = (e: KeyboardEvent, nudge: number) => {
      const next = getNext(activeItemIndex + nudge, items.length);
      setActiveItemIndex(next);
      // `false` - bottom of the element will be aligned to the bottom of the visible area of the scrollable ancestor
      activeItemRef.current?.scrollIntoView(false);
    };

    const handleKeyDown = (e: KeyboardEvent) => {
      switch (e.key) {
        case 'Enter': {
          e.stopPropagation();
          e.preventDefault();

          const item = items[activeItemIndex];
          if (item) {
            pickAndResetActiveItem(item);
          }
          break;
        }
        case 'Escape': {
          onBack();
          break;
        }
        case 'ArrowUp':
          e.stopPropagation();
          e.preventDefault();

          handleArrowKey(e, -1);
          break;
        case 'ArrowDown':
          e.stopPropagation();
          e.preventDefault();

          handleArrowKey(e, 1);
          break;
      }
    };

    const { cleanup } = addWindowEventListener('keydown', handleKeyDown, {
      capture: true,
    });
    return cleanup;
  }, [
    items,
    pickAndResetActiveItem,
    onBack,
    activeItemIndex,
    addWindowEventListener,
  ]);

  return (
    <>
      <Separator>
        {attempts.some(a => a.status === 'processing') && (
          <LinearProgress transparentBackground={true} />
        )}
      </Separator>
      <Overflow role="menu">
        {ExtraTopComponent}
        {items.map((r, index) => {
          const isActive = index === activeItemIndex;
          const { Component, key } = props.render(r);

          return (
            <InteractiveItem
              ref={isActive ? activeItemRef : null}
              role="menuitem"
              active={isActive}
              key={key}
              onClick={() => pickAndResetActiveItem(r)}
            >
              {Component}
            </InteractiveItem>
          );
        })}
      </Overflow>
    </>
  );
}

export const NonInteractiveItem = styled.div`
  & mark {
    color: inherit;
    background-color: rgba(0, 158, 255, 0.4); // Accent/Link at 40%
  }

  :not(:last-of-type) {
    border-bottom: 1px solid ${props => props.theme.colors.spotBackground[0]};
  }

  padding: ${props => props.theme.space[2]}px;
  color: ${props => props.theme.colors.text.main};
`;

const InteractiveItem = styled(NonInteractiveItem)`
  cursor: pointer;

  &:hover,
  &:focus {
    background: ${props => props.theme.colors.spotBackground[0]};
  }

  ${props => {
    if (props.active) {
      return css`
        background: ${props => props.theme.colors.spotBackground[0]};
      `;
    }
  }}
`;

/**
 * IconAndContent is supposed to be used within InteractiveItem & NonInteractiveItem.
 */
export function IconAndContent(
  props: React.PropsWithChildren<{
    Icon: React.ComponentType<{
      color: string;
      fontSize: string;
      lineHeight: string;
    }>;
    iconColor: string;
  }>
) {
  return (
    <Flex alignItems="flex-start" gap={2}>
      {/* lineHeight of the icon needs to match the line height of the first row of props.children */}
      <props.Icon color={props.iconColor} fontSize="20px" lineHeight="24px" />
      <Flex flexDirection="column" gap={1} minWidth={0} flex="1">
        {props.children}
      </Flex>
    </Flex>
  );
}

function getNext(selectedIndex = 0, max = 0) {
  let index = selectedIndex % max;
  if (index < 0) {
    index += max;
  }
  return index;
}

const Separator = styled.div`
  position: relative;
  background: ${props => props.theme.colors.spotBackground[0]};
  height: 1px;
`;

const Overflow = styled.div`
  overflow: auto;
  height: 100%;
  list-style: none outside none;
  max-height: 350px;
  // Hardcoded to height of the shortest item.
  min-height: 40px;
`;
