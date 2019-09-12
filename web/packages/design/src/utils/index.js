/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import ReactDOM from 'react-dom';
import getOffset from 'dom-helpers/query/offset';
import getPosition from 'dom-helpers/query/position';
import getScrollTop from 'dom-helpers/query/scrollTop';

export function ownerDocument(node) {
  return (node && node.ownerDocument) || document;
}

export function ownerWindow(node, fallback = window) {
  const doc = ownerDocument(node);
  return doc.defaultView || doc.parentView || fallback;
}

export function getContainer(container, defaultContainer){
  container = typeof container === 'function' ? container() : container;
  return ReactDOM.findDOMNode(container) || defaultContainer;
}

function getContainerDimensions(containerNode) {
  let width, height, scroll;

  if (containerNode.tagName === 'BODY') {
    width = window.innerWidth;
    height = window.innerHeight;

    scroll =
      getScrollTop(ownerDocument(containerNode).documentElement) ||
      getScrollTop(containerNode);
  } else {
    ({ width, height } = getOffset(containerNode));
    scroll = getScrollTop(containerNode);
  }

  return { width, height, scroll};
}

function getTopDelta(top, overlayHeight, container, padding) {
  const containerDimensions = getContainerDimensions(container);
  const containerScroll = containerDimensions.scroll;
  const containerHeight = containerDimensions.height;

  const topEdgeOffset = top - padding - containerScroll;
  const bottomEdgeOffset = top + padding - containerScroll + overlayHeight;

  if (topEdgeOffset < 0) {
    return -topEdgeOffset;
  } else if (bottomEdgeOffset > containerHeight) {
    return containerHeight - bottomEdgeOffset;
  } else {
    return 0;
  }
}

function getLeftDelta(left, overlayWidth, container, padding) {
  const containerDimensions = getContainerDimensions(container);
  const containerWidth = containerDimensions.width;

  const leftEdgeOffset = left - padding;
  const rightEdgeOffset = left + padding + overlayWidth;

  if (leftEdgeOffset < 0) {
    return -leftEdgeOffset;
  } else if (rightEdgeOffset > containerWidth) {
    return containerWidth - rightEdgeOffset;
  }

  return 0;
}

export function calculatePosition(placement, overlayNode, target, container, padding) {
  const childOffset = container.tagName === 'BODY' ?
    getOffset(target) : getPosition(target, container);

  const { height: overlayHeight, width: overlayWidth } =
    getOffset(overlayNode);

  let positionLeft, positionTop, arrowOffsetLeft, arrowOffsetTop;

  if (placement === 'left' || placement === 'right') {
    positionTop = childOffset.top + (childOffset.height - overlayHeight) / 2;

    if (placement === 'left') {
      positionLeft = childOffset.left - overlayWidth;
    } else {
      positionLeft = childOffset.left + childOffset.width;
    }

    const topDelta = getTopDelta(
      positionTop, overlayHeight, container, padding
    );

    positionTop += topDelta;
    arrowOffsetTop = 50 * (1 - 2 * topDelta / overlayHeight) + '%';
    arrowOffsetLeft = void 0;

  } else if (placement === 'top' || placement === 'bottom') {
    positionLeft = childOffset.left + (childOffset.width - overlayWidth) / 2;

    if (placement === 'top') {
      positionTop = childOffset.top - overlayHeight;
    } else {
      positionTop = childOffset.top + childOffset.height;
    }

    const leftDelta = getLeftDelta(
      positionLeft, overlayWidth, container, padding
    );

    positionLeft += leftDelta;
    arrowOffsetLeft = 50 * (1 - 2 * leftDelta / overlayWidth) + '%';
    arrowOffsetTop = void 0;

  } else {
    throw new Error(
      `calcOverlayPosition(): No such placement of "${placement}" found.`
    );
  }

  return { positionLeft, positionTop, arrowOffsetLeft, arrowOffsetTop };
}