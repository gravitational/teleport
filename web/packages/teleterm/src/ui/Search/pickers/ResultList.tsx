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

import React, {
  ReactElement,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';
import styled, { css } from 'styled-components';

import { Flex } from 'design';
import { IconProps } from 'design/Icon/Icon';
import { Attempt } from 'shared/hooks/useAsync';

import { LinearProgress } from 'teleterm/ui/components/LinearProgress';

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

  const items = attempts.map(a => a.data || []).flat();

  // Reset the active item index if it's greater than the number of available items.
  // This can happen in cases where the user selects the nth item and then filters the list so that
  // there's only one item.
  if (activeItemIndex !== 0 && activeItemIndex >= items.length) {
    setActiveItemIndex(0);
  }

  useEffect(() => {
    // This needs to happen directly in `useEffect` because it gives us a guarantee
    // that activeIndex matches the activeItemRef.
    activeItemRef.current?.scrollIntoView({ block: 'nearest' });

    const handleArrowKey = (e: KeyboardEvent, nudge: number) => {
      const next = getNext(activeItemIndex + nudge, items.length);
      setActiveItemIndex(next);
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

  &:not(:last-of-type) {
    border-bottom: 1px solid ${props => props.theme.colors.spotBackground[0]};
  }

  padding: ${props => props.theme.space[2]}px;
  color: ${props => props.theme.colors.text.main};
`;

const InteractiveItem = styled(NonInteractiveItem)<{ active?: boolean }>`
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
    Icon: React.ComponentType<IconProps>;
    iconColor: string;
    iconOpacity?: number;
  }>
) {
  return (
    <Flex alignItems="flex-start" gap={2}>
      {/* lineHeight of the icon needs to match the line height of the first row of props.children */}
      <Flex height="24px">
        <props.Icon
          color={props.iconColor}
          size="medium"
          style={{
            opacity: props.iconOpacity,
          }}
        />
      </Flex>
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
  // prevents showing a scrollbar when the container height is very low
  // by overriding our default line-height value
  line-height: normal;
`;
