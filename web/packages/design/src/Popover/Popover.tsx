/*
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

/*
The MIT License (MIT)

Copyright (c) 2014 Call-Em-All

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

import React, { createRef, MutableRefObject } from 'react';
import styled, { CSSProp, StyleFunction } from 'styled-components';

import Modal, { BackdropProps, Props as ModalProps } from '../Modal';

type Dimensions = { width: number; height: number };

export type Origin = {
  horizontal: HorizontalAnchor;
  vertical: VerticalAnchor;
};

export type HorizontalAnchor = 'left' | 'center' | 'right' | number;
export type VerticalAnchor = 'top' | 'center' | 'bottom' | number;
export type GrowDirections = 'top-left' | 'bottom-right';

type NumericOrigin = {
  horizontal: number;
  vertical: number;
};

function getOffsetTop(rect: Dimensions, vertical: VerticalAnchor): number {
  let offset = 0;

  if (typeof vertical === 'number') {
    offset = vertical;
  } else if (vertical === 'center') {
    offset = rect.height / 2;
  } else if (vertical === 'bottom') {
    offset = rect.height;
  }

  return offset;
}

function getOffsetLeft(rect: Dimensions, horizontal: HorizontalAnchor): number {
  let offset = 0;

  if (typeof horizontal === 'number') {
    offset = horizontal;
  } else if (horizontal === 'center') {
    offset = rect.width / 2;
  } else if (horizontal === 'right') {
    offset = rect.width;
  }

  return offset;
}

function getTransformOriginValue(transformOrigin: Origin): string {
  return [transformOrigin.horizontal, transformOrigin.vertical]
    .map(n => {
      return typeof n === 'number' ? `${n}px` : n;
    })
    .join(' ');
}

// Sum the scrollTop between two elements.
function getScrollParent(parent: Element, child: Element): number {
  let element = child;
  let scrollTop = 0;

  while (element && element !== parent) {
    element = element.parentElement;
    scrollTop += element.scrollTop;
  }
  return scrollTop;
}

function getAnchorEl(anchorEl: Element | (() => Element)): Element {
  return typeof anchorEl === 'function' ? anchorEl() : anchorEl;
}

export default class Popover extends React.Component<Props> {
  paperRef: MutableRefObject<HTMLDivElement> = createRef();
  handleResize: () => void;

  static defaultProps = {
    anchorReference: 'anchorEl',
    anchorOrigin: {
      vertical: 'top',
      horizontal: 'left',
    },
    marginThreshold: 16,
    transformOrigin: {
      vertical: 'top',
      horizontal: 'left',
    },
    growDirections: 'bottom-right',
  };

  constructor(props: Props) {
    super(props);

    if (typeof window !== 'undefined') {
      this.handleResize = () => {
        // Because we debounce the event, the open property might no longer be true
        // when the callback resolves.
        if (!this.props.open) {
          return;
        }

        this.setPositioningStyles(this.paperRef.current);
      };
    }
  }

  componentDidMount() {
    if (this.props.action) {
      this.props.action({
        updatePosition: this.handleResize,
      });
    }
  }

  setPositioningStyles = (element: HTMLElement) => {
    const positioning = this.getPositioningStyle(element);

    if (this.props.growDirections === 'bottom-right') {
      if (positioning.top !== null) {
        element.style.top = positioning.top;
      }
      if (positioning.left !== null) {
        element.style.left = positioning.left;
      }
    } else {
      if (positioning.bottom !== null) {
        element.style.bottom = positioning.bottom;
      }
      if (positioning.right !== null) {
        element.style.right = positioning.right;
      }
    }
    element.style.transformOrigin = positioning.transformOrigin;
  };

  getPositioningStyle = (element: HTMLElement) => {
    const { anchorReference, marginThreshold } = this.props;

    // Check if the parent has requested anchoring on an inner content node
    const contentAnchorOffset = this.getContentAnchorOffset(element);
    const elemRect = {
      width: element.offsetWidth,
      height: element.offsetHeight,
    };

    // Get the transform origin point on the element itself
    const transformOrigin = this.getTransformOrigin(
      elemRect,
      contentAnchorOffset
    );

    if (anchorReference === 'none') {
      return {
        top: null,
        left: null,
        transformOrigin: getTransformOriginValue(transformOrigin),
      };
    }

    // Get the offset of of the anchoring element
    const anchorOffset = this.getAnchorOffset(contentAnchorOffset);

    // Calculate element positioning
    let top = anchorOffset.top - transformOrigin.vertical;
    let left = anchorOffset.left - transformOrigin.horizontal;

    // bottom and right correspond to the calculated position of the element from the top left, not
    // from the bottom right, meaning they must be inverted before using them as `bottom` and
    // `right` CSS properties.
    let bottom = top + elemRect.height;
    let right = left + elemRect.width;

    // Window thresholds taking required margin into account
    const heightThreshold = window.innerHeight - marginThreshold;
    const widthThreshold = window.innerWidth - marginThreshold;

    // Check if the vertical axis needs shifting
    if (top < marginThreshold) {
      const diff = top - marginThreshold;
      top -= diff;
      transformOrigin.vertical += diff;
    } else if (bottom > heightThreshold) {
      const diff = bottom - heightThreshold;
      top -= diff;
      transformOrigin.vertical += diff;
    }

    // Check if the horizontal axis needs shifting
    if (left < marginThreshold) {
      const diff = left - marginThreshold;
      left -= diff;
      transformOrigin.horizontal += diff;
    } else if (right > widthThreshold) {
      const diff = right - widthThreshold;
      left -= diff;
      transformOrigin.horizontal += diff;
    }

    bottom = top + elemRect.height;
    right = left + elemRect.width;

    return {
      top: `${top}px`,
      left: `${left}px`,
      bottom: `${window.innerHeight - bottom}px`,
      right: `${window.innerWidth - right}px`,
      transformOrigin: getTransformOriginValue(transformOrigin),
    };
  };

  // Returns the top/left offset of the position
  // to attach to on the anchor element (or body if none is provided)
  getAnchorOffset(contentAnchorOffset: number): { top: number; left: number } {
    const { anchorEl, anchorOrigin } = this.props;

    // If an anchor element wasn't provided, just use the parent body element of this Popover
    const anchorElement = getAnchorEl(anchorEl) || document.body;

    const anchorRect = anchorElement.getBoundingClientRect();

    const anchorVertical =
      contentAnchorOffset === 0 ? anchorOrigin.vertical : 'center';

    return {
      top: anchorRect.top + getOffsetTop(anchorRect, anchorVertical),
      left:
        anchorRect.left + getOffsetLeft(anchorRect, anchorOrigin.horizontal),
    };
  }

  // Returns the vertical offset of inner content to anchor the transform on if provided
  getContentAnchorOffset(element: HTMLElement): number {
    const { getContentAnchorEl, anchorReference } = this.props;
    let contentAnchorOffset = 0;

    if (getContentAnchorEl && anchorReference === 'anchorEl') {
      const contentAnchorEl = getContentAnchorEl(element);

      if (contentAnchorEl && element.contains(contentAnchorEl)) {
        const scrollTop = getScrollParent(element, contentAnchorEl);
        contentAnchorOffset =
          contentAnchorEl.offsetTop +
            contentAnchorEl.clientHeight / 2 -
            scrollTop || 0;
      }
    }

    return contentAnchorOffset;
  }

  // Return the base transform origin using the element
  // and taking the content anchor offset into account if in use
  getTransformOrigin(
    elemRect: Dimensions,
    contentAnchorOffset = 0
  ): NumericOrigin {
    const { transformOrigin } = this.props;

    const vertical =
      getOffsetTop(elemRect, transformOrigin.vertical) + contentAnchorOffset;

    const horizontal = getOffsetLeft(elemRect, transformOrigin.horizontal);

    return {
      vertical,
      horizontal,
    };
  }

  handleEntering = (element: HTMLElement) => {
    if (this.props.onEntering) {
      this.props.onEntering(element);
    }

    this.setPositioningStyles(element);
  };

  setPaperRef = (el: HTMLDivElement) => {
    if (el && !this.paperRef.current) {
      this.handleEntering(el);
    }
    this.paperRef.current = el;
  };

  render() {
    const { children, open, popoverCss, ...other } = this.props;
    console.log(other);

    return (
      <Modal
        open={open}
        BackdropProps={{ invisible: true, ...this.props.backdropProps }}
        {...other}
      >
        <StyledPopover
          popoverCss={popoverCss}
          data-mui-test="Popover"
          ref={this.setPaperRef}
        >
          {children}
        </StyledPopover>
      </Modal>
    );
  }
}

interface Props extends Omit<ModalProps, 'children' | 'open'> {
  /**
   * This is callback property. It's called by the component on mount.  This is
   * useful when you want to trigger an action programmatically.  It currently
   * only supports updatePosition() action.
   *
   * @param actions This object contains all possible actions that can be
   * triggered programmatically.
   */
  action?: (actions: { updatePosition: () => void }) => void;

  /**
   * This is the DOM element, or a function that returns the DOM element, that
   * may be used to set the position of the popover.
   */
  anchorEl?: Element | (() => Element);

  /**
   * This is the point on the anchor where the popover's `anchorEl` will attach
   * to.
   */
  anchorOrigin?: Origin;

  /**
   * These are the directions in which `Popover` will grow if its content
   * increases its dimensions after `Popover` is opened.
   */
  growDirections?: GrowDirections;

  /**
   * This determines which anchor prop to refer to to set the position of the
   * popover.
   */
  anchorReference?: 'anchorEl' | 'none';

  /**
   * The content of the component.
   */
  children?: React.ReactNode;

  /**
   * This function is called in order to retrieve the content anchor element.
   * It's the opposite of the `anchorEl` property.  The content anchor element
   * should be an element inside the popover.  It's used to correctly scroll and
   * set the position of the popover.  The positioning strategy tries to make
   * the content anchor element just above the anchor element.
   */
  getContentAnchorEl?: (paperElement: HTMLElement) => HTMLElement;

  /**
   * Specifies how close to the edge of the window the popover can appear.
   */
  marginThreshold?: number;

  /**
   * Callback fired when the component requests to be closed.
   *
   * @param event The event source of the callback.
   * @param reason Can be:`"escapeKeyDown"`, `"backdropClick"`
   */
  onClose?: (
    event: React.MouseEvent | KeyboardEvent,
    reason: 'escapeKeyDown' | 'backdropClick'
  ) => void;

  /**
   * Callback fired when the component is entering.
   */
  onEntering?: (paperElement: HTMLElement) => void;

  /**
   * If `true`, the popover is visible.
   */
  open: boolean;

  /**
   * This is the point on the popover which will attach to the anchor's origin.
   *
   * Options:
   * vertical: [top, center, bottom, x(px)];
   * horizontal: [left, center, right, x(px)].
   */
  transformOrigin?: Origin;

  /** Returns additional styles applied to the internal popover element. */
  popoverCss?: () => CSSProp;

  /** Properties applied to the backdrop element. */
  backdropProps?: BackdropProps;
}

export const StyledPopover = styled.div<{ popoverCss?: () => CSSProp }>`
  box-shadow: ${props => props.theme.boxShadow[1]};
  border-radius: 4px;
  max-width: calc(100% - 32px);
  max-height: calc(100% - 32px);
  min-height: 16px;
  min-width: 16px;
  outline: none;
  overflow-x: hidden;
  overflow-y: auto;
  position: absolute;
  ${props => props.popoverCss && props.popoverCss()}
`;
