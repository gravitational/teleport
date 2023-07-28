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

import React, { useRef } from 'react';

// useSlidingHover provides a hook that can be used to create a background that slides between
// elements on hover. It takes an argument for the delay in milliseconds for the transition.
//
// parentRef is for the parent element that contains children that will have their hover state
// handled by this hook. The parent element should have position: relative.
//
// ref is for the sliding background element. It should have position: absolute.
//
// handleMouseLeave is for the onMouseLeave event handler for the parent element.
// handleMouseEnter is for the onMouseEnter event handler for the children elements.
//
// Example usage:
//
// const {
//   ref,
//   parentRef,
//   handleMouseLeave,
//   handleMouseEnter,
// } = useSlidingHover(200);
//
// return (
//   <TabsContainer ref={parentRef} onMouseLeave={handleMouseLeave}>
//     <Tab onMouseEnter={handleMouseEnter}>Item 1</div>
//     <Tab onMouseEnter={handleMouseEnter}>Item 2</div>
//     <Tab onMouseEnter={handleMouseEnter}>Item 3</div>
//
//     <TabBackground ref={ref} />
//   </div>
// );
export function useSlidingHover(delayMs: number) {
  const ref = useRef<HTMLElement>();
  const parentRef = useRef<HTMLElement>();
  const hasMouseOver = useRef(false);

  function handleMouseLeave() {
    ref.current.style.opacity = '0';
    hasMouseOver.current = false;
  }

  function handleMouseEnter(e: React.MouseEvent<HTMLDivElement>) {
    if (!ref.current || !parentRef.current) {
      return;
    }

    const boundingRect = e.currentTarget.getBoundingClientRect();
    const parentRect = parentRef.current.getBoundingClientRect();

    ref.current.style.transitionDuration = hasMouseOver.current
      ? `${delayMs}ms`
      : '0s';

    ref.current.style.height = `${e.currentTarget.clientHeight}px`;
    ref.current.style.width = `${e.currentTarget.clientWidth}px`;

    ref.current.style.left = `${boundingRect.x - parentRect.x}px`;
    ref.current.style.top = `${boundingRect.y - parentRect.y}px`;

    ref.current.style.opacity = '1';
    hasMouseOver.current = true;
  }

  return {
    ref,
    parentRef,
    handleMouseLeave,
    handleMouseEnter,
  };
}
