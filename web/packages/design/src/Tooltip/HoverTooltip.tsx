/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import {
  arrow,
  autoUpdate,
  flip,
  FloatingArrow,
  FloatingPortal,
  offset,
  shift,
  useDismiss,
  useFloating,
  useFocus,
  useHover,
  useInteractions,
  useRole,
  useTransitionStyles,
  type Placement,
} from '@floating-ui/react';
import React, {
  Children,
  cloneElement,
  isValidElement,
  ReactElement,
  Ref,
  useRef,
  useState,
} from 'react';
import styled, { useTheme } from 'styled-components';

import Text from 'design/Text';

type HoverTooltipProps = {
  /**
   * String or ReactNode to display in tooltip.
   */
  tipContent?: React.ReactNode;
  /**
   * Only show tooltip if trigger content is overflowing its container.
   */
  showOnlyOnOverflow?: boolean;
  /**
   * Specifies the position of tooltip relative to trigger content.
   */
  placement?: Placement;
  /**
   * @deprecated – Prefer specifying `placement` instead.
   */
  position?: Placement;
  /**
   * Offset the tooltip relative to trigger content. Defaults to `8`.
   */
  offset?: number;
  /**
   * Delay opening and/or closing of the tooltip.
   */
  delay?: number | { open: number; close: number };
  /**
   * Don't flip the tooltip's placement when tooltip runs out of the viewport.
   */
  disableFlip?: boolean;
  /**
   * Don't transition the tooltip in/out on mount/unmount.
   */
  disableTransitions?: boolean;
  /**
   * Child to render. The type allows only a single child.
   */
  children?: ReactElement;
};

/**
 * Renders a tooltip on hover.
 *
 * The tooltip is anchored to the child element via a ref.
 * Therefore, the child **must** be a single React element that accepts a `ref`.
 * If the child cannot accept a ref, the tooltip will not be displayed.
 */
export const HoverTooltip = ({
  tipContent,
  children,
  showOnlyOnOverflow = false,
  placement = 'top',
  position,
  offset: offsetDistance = 8,
  delay = 0,
  disableFlip = false,
  disableTransitions = false,
}: HoverTooltipProps) => {
  const theme = useTheme();
  const [open, setOpen] = useState(false);
  const arrowRef = useRef(null);

  if (position) {
    placement = position;
  }

  const handleOpenChange = (open: boolean, event?: Event) => {
    if (!open) {
      setOpen(false);
      return;
    }

    if (!showOnlyOnOverflow) {
      setOpen(true);
      return;
    }

    if (!(event?.currentTarget instanceof HTMLElement)) {
      return;
    }

    const target = event.currentTarget;
    const parent = target?.parentElement;

    const isOverflowing =
      target && parent && target.scrollWidth > parent.offsetWidth;
    if (!isOverflowing) {
      return;
    }
    setOpen(true);
  };

  const { x, y, strategy, refs, context } = useFloating({
    placement,
    open,
    onOpenChange: handleOpenChange,
    middleware: [
      offset(offsetDistance),
      !disableFlip && flip(),
      shift({ padding: 8 }),
      arrow({ element: arrowRef }),
    ].filter(Boolean),
    whileElementsMounted: autoUpdate,
  });

  const { isMounted, styles: transitionStyles } = useTransitionStyles(context, {
    duration: 100,
    initial: {
      opacity: '0',
      transform: 'scale(0.96)',
    },
    common: ({ side }) => ({
      transformOrigin: {
        top: 'bottom',
        bottom: 'top',
        left: 'right',
        right: 'left',
      }[side],
      transitionTimingFunction: 'ease',
    }),
  });

  const openDelay = typeof delay === 'object' ? delay.open : delay;
  const closeDelay = typeof delay === 'object' ? delay.close : delay;

  const { getFloatingProps } = useInteractions([
    useHover(context, {
      delay: { open: openDelay, close: closeDelay },
      handleClose: null,
    }),
    useFocus(context),
    useDismiss(context),
    useRole(context, { role: 'tooltip' }),
  ]);

  if (!tipContent || !children) {
    return <>{children}</>;
  }

  // The type of `children` is `ReactElement` which allows only one child.
  let child = Children.only(children);
  if (isValidElement(child)) {
    const originalRef = (child.props as { ref?: Ref<HTMLElement> }).ref;

    child = cloneElement(child, {
      // @ts-expect-error we don't know the child type.
      ref: mergeRefs([refs.setReference, originalRef]),
    });
  }

  return (
    <>
      {child}
      {isMounted && (
        <FloatingPortal>
          <StyledTooltip
            ref={refs.setFloating}
            style={{
              position: strategy,
              top: y ?? 0,
              left: x ?? 0,
              background: theme.colors.tooltip.background,
              backdropFilter: 'blur(2px)',
              color: theme.colors.text.primaryInverse,
              ...(!disableTransitions ? transitionStyles : { opacity: 1 }),
            }}
            {...getFloatingProps()}
          >
            <FloatingArrow
              ref={arrowRef}
              context={context}
              style={{
                fill: theme.colors.tooltip.background,
                backdropFilter: 'blur(2px)',
              }}
            />
            <StyledContent px={3} py={2}>
              {tipContent}
            </StyledContent>
          </StyledTooltip>
        </FloatingPortal>
      )}
    </>
  );
};

const StyledTooltip = styled.div`
  max-width: 350px;
  word-wrap: break-word;
  z-index: 1500;
  border-radius: 4px;
  pointer-events: none;
  filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.15));
  backdrop-filter: blur(2px);
`;

const StyledContent = styled(Text)`
  max-width: 350px;
  word-wrap: break-word;
`;

/**
 * Combines multiple refs into one that can be passed to a component.
 *
 * https://github.com/gregberge/react-merge-refs/tree/v2.1.1
 * @example
 * const Example = React.forwardRef(function Example(props, ref) {
 *   const localRef = React.useRef();
 *   return <div ref={mergeRefs([localRef, ref])} />;
 * });
 */
function mergeRefs<T = any>(
  refs: Array<React.RefObject<T> | React.Ref<T> | undefined | null>
): React.RefCallback<T> {
  return value => {
    refs.forEach(ref => {
      if (typeof ref === 'function') {
        ref(value);
      } else if (ref != null) {
        (ref as React.RefObject<T | null>).current = value;
      }
    });
  };
}
