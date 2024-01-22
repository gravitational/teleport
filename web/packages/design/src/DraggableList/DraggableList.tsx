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
  Children,
  PropsWithChildren,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import styled from 'styled-components';

import { animated, to, useSprings } from '@react-spring/web';
import { useGesture } from '@use-gesture/react';
import { ReactDOMAttributes } from '@use-gesture/react/dist/declarations/src/types';

const Container = styled.div`
  position: relative;
`;

interface DraggableListProps {
  onOrderChange: (newOrder: number[]) => void;
}

export function DraggableList(props: PropsWithChildren<DraggableListProps>) {
  const children = Children.toArray(props.children);

  const [childrenHeight, setChildrenHeight] = useState(0);

  const animatedDiv = useRef<HTMLDivElement>(null);
  const order = useRef<number[]>(children.map((_, index) => index));

  function createPropsGetter(
    orderList: number[] = order.current,
    down?: boolean,
    originalIndex?: number,
    newHeight?: number
  ) {
    return function getProps(index: number) {
      if (down && index === originalIndex) {
        return {
          height: newHeight,
          width: 0,
          zIndex: 1,
          immediate: (key: string) =>
            key === 'active' || key === 'height' || key === 'zIndex',
        };
      }

      return {
        height: childrenHeight * orderList.indexOf(index),
        width: 0,
        zIndex: 0,
        immediate: false,
      };
    };
  }

  const [springs, api] = useSprings(children.length, createPropsGetter());

  const bind = useGesture({
    onDrag: state => {
      const [, offset] = state.movement;
      if (offset === 0) {
        return;
      }

      const [originalIndex] = state.args;

      const curIndex = order.current.indexOf(originalIndex);
      const newHeight = childrenHeight * curIndex + offset;
      const nextIndex = clamp(
        Math.round(newHeight / childrenHeight),
        0,
        children.length - 1
      );
      const newOrder = swap(order.current, curIndex, nextIndex);

      api.start(
        createPropsGetter(newOrder, state.down, originalIndex, newHeight)
      );
      if (!state.down) {
        order.current = newOrder;
      }
    },
    onDragEnd: () => {
      api.start(createPropsGetter());
      props.onOrderChange(order.current);
    },
  }) as unknown as (...args: any[]) => ReactDOMAttributes; // useGesture typings are wrong https://github.com/pmndrs/use-gesture/issues/362

  useEffect(() => {
    order.current = children.map((_, index) => index);

    api.start(createPropsGetter());
  }, [children.length]);

  useLayoutEffect(() => {
    if (!animatedDiv.current) {
      return;
    }

    const height = animatedDiv.current.scrollHeight;
    if (childrenHeight !== height) {
      setChildrenHeight(height);

      return;
    }

    api.start(createPropsGetter());
  }, [api, childrenHeight, children.length]);

  return (
    <Container
      style={{ height: childrenHeight * (springs.length + 1) }}
      onClick={e => e.stopPropagation()}
    >
      {springs.map((spring, index) => {
        const { width, height, zIndex } = spring;

        return (
          <animated.div
            ref={animatedDiv}
            {...bind(index)}
            key={index}
            style={{
              position: 'absolute',
              zIndex,
              width: '100%',
              transform: to(
                [width, height],
                (x, y) => `translate3d(${x}px, ${y}px, 0)`
              ),
              touchAction: 'none',
            }}
          >
            {children[index]}
          </animated.div>
        );
      })}
    </Container>
  );
}

function clamp(pos: number, low: number, high: number) {
  const mid = Math.max(pos, low);

  return Math.min(mid, high);
}

function swap<T>(arr: T[], a: number, b: number): T[] {
  const copy = [...arr];
  const [index] = copy.splice(a, 1);

  copy.splice(b, 0, index);

  return copy;
}
