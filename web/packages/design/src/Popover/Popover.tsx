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

import React, { Component, createRef } from 'react';
import styled, { CSSProp } from 'styled-components';

import Flex from 'design/Flex';

import Modal, { BackdropProps, ModalProps } from '../Modal';
import { Transition } from './Transition';

type Offset = { top: number; left: number };
type Dimensions = { width: number; height: number };

export type Origin = {
  horizontal: HorizontalOrigin;
  vertical: VerticalOrigin;
};

export type HorizontalOrigin = 'left' | 'center' | 'right' | number;
export type VerticalOrigin = 'top' | 'center' | 'bottom' | number;
export type GrowDirections = 'top-left' | 'bottom-right';
export type Position = 'top' | 'right' | 'bottom' | 'left';

type NumericOrigin = {
  horizontal: number;
  vertical: number;
};

function getOffsetTop(rect: Dimensions, vertical: VerticalOrigin): number {
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

function getOffsetLeft(rect: Dimensions, horizontal: HorizontalOrigin): number {
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

/**
 * Returns popover position, relative to the anchor. If unambiguously defined by
 * the transform origin, returns this one. The ambiguous cases (transform origin
 * on one of the popover corners) are resolved by looking into the anchor
 * origin. If still ambiguous (corner touching a corner), prefers a vertical
 * position.
 */
function getPopoverPosition(
  anchorOrigin: Origin,
  transformOrigin: Origin
): Position | null {
  const allowedByTransformOrigin = getAllowedPopoverPositions(transformOrigin);
  switch (allowedByTransformOrigin.length) {
    case 0:
      return null;
    case 1:
      return allowedByTransformOrigin[0];

    default: {
      const preferredByAnchorOrigin =
        getPreferredPopoverPositions(anchorOrigin);
      const resolved = allowedByTransformOrigin.filter(d =>
        preferredByAnchorOrigin.includes(d)
      );
      if (resolved.length === 0) return null;
      return resolved[0];
    }
  }
}

/** Returns popover positions allowed by the transform origin. */
function getAllowedPopoverPositions(transformOrigin: Origin) {
  const allowed: Position[] = [];
  // Note: order matters here. The first one will be preferred when no
  // unambiguous decision is reached, so we arbitrarily prefer vertical over
  // horizontal arrows.
  if (transformOrigin.vertical === 'top') allowed.push('bottom');
  if (transformOrigin.vertical === 'bottom') allowed.push('top');
  if (transformOrigin.horizontal === 'left') allowed.push('right');
  if (transformOrigin.horizontal === 'right') allowed.push('left');
  return allowed;
}

/** Returns popover positions preferred by the anchor origin. */
function getPreferredPopoverPositions(anchorOrigin: Origin) {
  const preferred: Position[] = [];
  if (anchorOrigin.vertical === 'top') preferred.push('top');
  if (anchorOrigin.vertical === 'bottom') preferred.push('bottom');
  if (anchorOrigin.horizontal === 'left') preferred.push('left');
  if (anchorOrigin.horizontal === 'right') preferred.push('right');
  return preferred;
}

/** Returns vertical position adjustment resulting from the popover margin. */
function getPopoverMarginTop(
  popoverPos: Position | null,
  popoverMargin: number
): number {
  if (popoverPos === 'top') return popoverMargin;
  if (popoverPos === 'bottom') return -popoverMargin;
  return 0;
}

/** Returns horizontal position adjustment resulting from the popover margin. */
function getPopoverMarginLeft(
  popoverPos: Position | null,
  popoverMargin: number
): number {
  if (popoverPos === 'left') return popoverMargin;
  if (popoverPos === 'right') return -popoverMargin;
  return 0;
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
  let element: Element | null = child;
  let scrollTop = 0;

  while (element && element !== parent) {
    element = element.parentElement;
    if (!element) {
      break;
    }
    scrollTop += element.scrollTop;
  }
  return scrollTop;
}

function getAnchorEl(anchorEl: Element | (() => Element)): Element {
  return typeof anchorEl === 'function' ? anchorEl() : anchorEl;
}

/** Distance in pixels from the base to the tip of the arrow. */
const arrowLength = 8;
/** Distance in pixels between the arrow arms at the arrow base. */
const arrowWidth = 2 * arrowLength;
const borderRadius = 4;

// Attention: advanced CSS magic below.
//
// We need to support transparency, blur filters, round corners, arrow tooltips.
// The only technique that allows us to meet these criteria and make sure that
// the arrow doesn't look like a broken glass shard when displayed on a
// non-uniform background, is to use mask images.
//
// The following code implements and extends the technique described in
// https://css-tricks.com/perfect-tooltips-with-css-clipping-and-masking/#aa-more-complex-shapes.
// If you want to modify it, always observe these rules:
//
// 1. Keep the same number of elements in all of mask-image, mask-size, and
//    mask-position declarations.
// 2. Pay close attention to the syntax, particularly to the commas separating
//    the elements.
// 3. Make changes incrementally and test them live.
//
// The last rule is particularly important, since the style parser is
// unforgiving and won't tell you what's wrong with your declaration, so the
// best way to find where your mistake is is to only do one thing at a time.
//
// The following functions return an image mask that draws a tooltip shape using
// following elements:
//
// 1. Four circles that render the round corners. Note that since browsers don't
//    use anti-aliasing on gradients, we are making a 0.5px transition layer
//    between transparent and opaque region. It's enough to force anti-aliasing
//    on a regular density display. On a retina display, 1px would make the
//    corners blurry.
// 2. Two rectangles that fill the spaces between the round corners, first one
//    horizontally, the second one vertically.
// 3. A simple, three-point polygon that draws an arrow shape. Note that due to
//    a quirk that prevents us from using the mask-position attribute with SVG
//    documents, we can't just render it and refer to it by ID; the polygon
//    needs to be literally embedded in the style definition.

/**
 * Returns a mask-image declaration that includes arrow polygon points. It's
 * variable, since we can't rotate it, so each arrow direction gets its own
 * polygon.
 */
function getMaskImage(arrowPolygonPoints: string) {
  return `
    radial-gradient(#fff ${borderRadius - 0.5}px, #fff0 ${borderRadius}px),
    radial-gradient(#fff ${borderRadius - 0.5}px, #fff0 ${borderRadius}px),
    radial-gradient(#fff ${borderRadius - 0.5}px, #fff0 ${borderRadius}px),
    radial-gradient(#fff ${borderRadius - 0.5}px, #fff0 ${borderRadius}px),

    linear-gradient(#fff, #fff),
    linear-gradient(#fff, #fff),

    url('data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg"><polygon points="${arrowPolygonPoints}"/></svg>')
    `;
}

/**
 * Returns four constant sizes for the corner masks and adds a variable portion
 * for the space between corners and the arrow.
 */
function getMaskSize(variableSizes: string) {
  return `
    ${2 * borderRadius}px ${2 * borderRadius}px,
    ${2 * borderRadius}px ${2 * borderRadius}px,
    ${2 * borderRadius}px ${2 * borderRadius}px,
    ${2 * borderRadius}px ${2 * borderRadius}px,
    ${variableSizes}
  `;
}

/**
 * A set of `mask-image` and related styles to be applied to the popover element
 * in order to "carve out" a shape of tooltip with an arrow.
 */
type MaskStyles = {
  maskImage: string;
  maskPosition: string;
  maskSize: string;
};

const noMaskStyles = {
  maskImage: '',
  maskPosition: '',
  maskSize: '',
};

/**
 * Returns mask styles to be applied to element with a given popover relative
 * position and arrow coordinates.
 * @param arrow Determines whether an arrow should be shown at all; if it's not
 *   necessary, we don't need any mask at all.
 * @param popoverPos Determines the direction of popover in relation to the
 *   anchor.
 * @param arrowLeft Horizontal position of the arrow, ignored if the arrow is
 *   horizontal.
 * @param arrowTop Vertical position of the arrow, ignored if the arrow is
 *   vertical.
 */
function getMaskStyles(
  arrow: boolean,
  popoverPos: Position | null,
  arrowLeft: number,
  arrowTop: number
): MaskStyles {
  if (!arrow) {
    return noMaskStyles;
  }

  switch (popoverPos) {
    case 'top':
      return {
        // Mask image with specific arrow polygon points, corresponding to the
        // arrow direction.
        maskImage: getMaskImage(
          `0 0, ${arrowWidth / 2} ${arrowLength}, ${arrowWidth} 0`
        ),
        // Circles in four corners, then rectangles pinned to the left and top
        // edges of the tooltip, respectively. Last but not least, the arrow tip
        // position.
        maskPosition: `
          0 0,
          100% 0,
          100% calc(100% - ${arrowLength}px),
          0 calc(100% - ${arrowLength}px),

          0 ${borderRadius}px,
          ${borderRadius}px 0,

          ${arrowLeft}px 100%
        `,
        // Constant sizes for the circles (see getMaskSize), and then rectangles
        // sized according to the size of the tooltip. Arrow dimensions depend
        // only on its orientation.
        maskSize: getMaskSize(`
          100% calc(100% - ${arrowLength + 2 * borderRadius}px),
          calc(100% - ${2 * borderRadius}px) calc(100% - ${arrowLength}px),

          ${arrowWidth}px ${arrowLength}px
        `),
      };
    case 'right':
      return {
        maskImage: getMaskImage(
          `${arrowLength} 0, 0 ${arrowWidth / 2}, ${arrowLength} ${arrowWidth}`
        ),
        maskPosition: `
          ${arrowLength}px 0,
          100% 0,
          100% 100%,
          ${arrowLength}px 100%,

          ${arrowLength}px ${borderRadius}px,
          ${arrowLength + borderRadius}px 0,

          0 ${arrowTop}px
        `,
        maskSize: getMaskSize(`
          calc(100% - ${arrowLength}px) calc(100% - ${2 * borderRadius}px),
          calc(100% - ${arrowLength + 2 * borderRadius}px) 100%,

          ${arrowLength}px ${arrowWidth}px
        `),
      };
    case 'bottom':
      return {
        maskImage: getMaskImage(
          `0 ${arrowLength}, ${arrowWidth / 2} 0, ${arrowWidth} ${arrowLength}`
        ),
        maskPosition: `
          0 ${arrowLength}px,
          100% ${arrowLength}px,
          100% 100%,
          0 100%,

          0 ${arrowLength + borderRadius}px,
          ${borderRadius}px ${arrowLength}px,

          ${arrowLeft}px 0
        `,
        maskSize: getMaskSize(`
          100% calc(100% - ${arrowLength + 2 * borderRadius}px),
          calc(100% - ${2 * borderRadius}px) calc(100% - ${arrowLength}px),

          ${arrowWidth}px ${arrowLength}px
        `),
      };
    case 'left':
      return {
        maskImage: getMaskImage(
          `0 0, ${arrowLength} ${arrowWidth / 2}, 0 ${arrowWidth}`
        ),
        maskPosition: `
          0 0,
          calc(100% - ${arrowLength}px) 0,
          calc(100% - ${arrowLength}px) 100%,
          0 100%,

          0 ${borderRadius}px,
          ${borderRadius}px 0,

          100% ${arrowTop}px
        `,
        maskSize: getMaskSize(`
          calc(100% - ${arrowLength}px) calc(100% - ${2 * borderRadius}px),
          calc(100% - ${arrowLength + 2 * borderRadius}px) 100%,

          ${arrowLength}px ${arrowWidth}px
        `),
      };
    default:
      return noMaskStyles;
  }
}

/**
 * Returns a CSS prop name that will receive additional padding related to the
 * arrow position. This padding is necessary, since we manually draw the
 * tooltip shape with an arrow. The tooltip element covers the entire area
 * along with the arrow, and we push its content so that it doesn't enter the
 * area reserved for the arrow.
 */
function getArrowPaddingProp(
  popoverPos: Position | null
): 'paddingBottom' | 'paddingLeft' | 'paddingTop' | 'paddingRight' | null {
  switch (popoverPos) {
    case null:
      return null;
    case 'top':
      return 'paddingBottom';
    case 'right':
      return 'paddingLeft';
    case 'bottom':
      return 'paddingTop';
    case 'left':
      return 'paddingRight';
  }
}

export class Popover extends Component<Props> {
  paperRef = createRef<HTMLDivElement>();
  handleResize: () => void = () => {};

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
    arrow: false,
    popoverMargin: 0,
    arrowMargin: 4,
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

        this.setPositioningStyles();
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

  setPositioningStyles = () => {
    const paper = this.paperRef.current;

    if (!paper) {
      return;
    }

    const popoverPos = getPopoverPosition(
      // We use the non-null assertion operator (!) here and elsewhere to tell TS
      // that the value is guaranteed to be defined due to default props.
      // Unfortunately, `defaultProps` field is not recognized by TS, so assertions are needed.
      // This approach is a workaround and is not recommended, as we lose the benefits of strict null checks.
      this.props.anchorOrigin!,
      this.props.transformOrigin!
    );

    const arrowPaddingProp = getArrowPaddingProp(popoverPos);
    if (arrowPaddingProp && this.props.arrow) {
      paper.style[arrowPaddingProp] = `${arrowLength}px`;
    } else {
      paper.style.padding = '0';
    }

    const {
      top,
      left,
      bottom,
      right,
      transformOrigin,
      maskImage,
      maskPosition,
      maskSize,
    } = this.getPositioningStyle(paper);

    if (this.props.growDirections === 'bottom-right') {
      if (top !== undefined) {
        paper.style.top = top;
      }
      if (left !== undefined) {
        paper.style.left = left;
      }
    } else {
      if (bottom !== undefined) {
        paper.style.bottom = bottom;
      }
      if (right !== undefined) {
        paper.style.right = right;
      }
    }
    paper.style.transformOrigin = transformOrigin;
    paper.style.maskImage = maskImage;
    paper.style.maskPosition = maskPosition;
    paper.style.maskSize = maskSize;
  };

  getPositioningStyle = (
    element: HTMLDivElement
  ): {
    top?: string;
    left?: string;
    bottom?: string;
    right?: string;
    transformOrigin: string;
  } & MaskStyles => {
    const anchorReference = this.props.anchorReference!;
    const marginThreshold = this.props.marginThreshold!;
    const arrowMargin = this.props.arrowMargin!;

    // Check if the parent has requested anchoring on an inner content node
    const contentAnchorOffset = this.getContentAnchorOffset(element);
    const elemRect = element.getBoundingClientRect();

    // Get the transform origin point on the element itself
    const transformOrigin = this.getTransformOrigin(
      elemRect,
      contentAnchorOffset
    );

    if (anchorReference === 'none') {
      return {
        top: undefined,
        left: undefined,
        transformOrigin: getTransformOriginValue(transformOrigin),
        ...noMaskStyles,
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

    const popoverPos = getPopoverPosition(
      this.props.anchorOrigin!,
      this.props.transformOrigin!
    );

    // Calculate the arrow position.
    let arrowLeft = 0;
    let arrowTop = 0;
    switch (popoverPos) {
      case 'left':
      case 'right':
        arrowTop = transformOrigin.vertical - arrowWidth / 2;
        if (arrowTop < arrowMargin) {
          arrowTop = arrowMargin;
        }
        if (arrowTop > elemRect.height - arrowWidth - arrowMargin) {
          arrowTop = elemRect.height - arrowWidth - arrowMargin;
        }
        break;
      case 'top':
      case 'bottom':
        arrowLeft = transformOrigin.horizontal - arrowWidth / 2;
        if (arrowLeft < arrowMargin) {
          arrowLeft = arrowMargin;
        }
        if (arrowLeft > elemRect.width - arrowWidth - arrowMargin) {
          arrowLeft = elemRect.width - arrowWidth - arrowMargin;
        }
        break;
    }

    return {
      top: `${top}px`,
      left: `${left}px`,
      bottom: `${window.innerHeight - bottom}px`,
      right: `${window.innerWidth - right}px`,
      transformOrigin: getTransformOriginValue(transformOrigin),
      ...getMaskStyles(this.props.arrow!, popoverPos, arrowLeft, arrowTop),
    };
  };

  // Returns the top/left offset of the position
  // to attach to on the anchor element (or body if none is provided)
  getAnchorOffset(contentAnchorOffset: number): Offset {
    const { anchorEl, anchorOrigin } = this.props;

    // If an anchor element wasn't provided, just use the parent body element of this Popover
    const anchorElement = getAnchorEl(anchorEl!) || document.body;

    const anchorRect = anchorElement.getBoundingClientRect();

    const anchorVertical =
      contentAnchorOffset === 0 ? anchorOrigin!.vertical : 'center';

    return {
      top: anchorRect.top + getOffsetTop(anchorRect, anchorVertical),
      left:
        anchorRect.left + getOffsetLeft(anchorRect, anchorOrigin!.horizontal),
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

    const popoverPos = getPopoverPosition(
      this.props.anchorOrigin!,
      this.props.transformOrigin!
    );

    const vertical =
      getOffsetTop(elemRect, transformOrigin!.vertical) +
      getPopoverMarginTop(popoverPos, this.props.popoverMargin!) +
      contentAnchorOffset;

    const horizontal =
      getOffsetLeft(elemRect, transformOrigin!.horizontal) +
      getPopoverMarginLeft(popoverPos, this.props.popoverMargin!);

    return {
      vertical,
      horizontal,
    };
  }

  handleEntering = () => {
    if (this.props.onEntering && this.paperRef.current) {
      this.props.onEntering(this.paperRef.current);
    }

    this.setPositioningStyles();
  };

  render() {
    const { children, open, popoverCss, ...other } = this.props;

    return (
      <Modal
        open={open}
        BackdropProps={{ invisible: true, ...this.props.backdropProps }}
        {...other}
      >
        <Transition
          onEntering={this.handleEntering}
          enablePaperResizeObserver={this.props.updatePositionOnChildResize}
          paperRef={this.paperRef}
          onPaperResize={this.setPositioningStyles}
        >
          <StyledPopover
            shadow={true}
            popoverCss={popoverCss}
            data-mui-test="Popover"
            ref={this.paperRef}
            style={{
              maskRepeat: 'no-repeat',
            }}
          >
            {children}
          </StyledPopover>
        </Transition>
      </Modal>
    );
  }
}

export default Popover;

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
  anchorEl?: Element | (() => Element) | null;

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

  /** `true` indicates an arrow will be displayed pointing at the anchor. */
  arrow?: boolean;

  /** Distance between anchor and the popover. */
  popoverMargin?: number;

  /**
   * Minimum distance between an arrow and the edge of the popover. Important
   * for proper rendering of rounded corners which should not interfere with
   * arrow tips.
   */
  arrowMargin?: number;

  /**
   * If false (default), positioning styles are updated only on the initial render of the children.
   *
   * If true, updates positioning styles of the popover whenever the children are resized.
   * This is useful in situations where the children are updated asynchronously, e.g., after
   * receiving a response over network.
   */
  updatePositionOnChildResize?: boolean;
}

export const StyledPopover = styled(Flex)<{
  shadow: boolean;
  popoverCss?: () => CSSProp;
}>`
  /* Ignored if we apply the mask. */
  box-shadow: ${props => (props.shadow ? props.theme.boxShadow[1] : 'none')};
  border-radius: ${borderRadius}px;

  background-color: ${props => props.theme.colors.levels.elevated};
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
