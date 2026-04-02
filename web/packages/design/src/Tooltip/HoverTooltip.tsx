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
  type Placement,
  safePolygon,
  shift,
  useClick,
  useDismiss,
  useFloating,
  useFocus,
  useHover,
  useInteractions,
  useRole,
  useTransitionStyles,
} from '@floating-ui/react';
import React, {
  Children,
  cloneElement,
  ReactElement,
  Ref,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import styled, { useTheme } from 'styled-components';

import Text from 'design/Text';
import { mergeRefs } from 'design/utils/mergeRefs';

type HoverTooltipProps = {
  /**
   * String or ReactNode to display in tooltip.
   */
  tipContent?: React.ReactNode;
  /**
   * Prevent the tooltip from opening. If the tooltip is already open, it will close.
   */
  disabled?: boolean;
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
  children: ReactElement<{ ref: Ref<HTMLElement> }>;
  /**
   * How the tooltip is triggered. Defaults to `'hover'`.
   */
  trigger?: 'hover' | 'click';
  /**
   * When true, the tooltip stays open while the user hovers over it,
   * allowing interaction with its content.
   */
  sticky?: boolean;
  /**
   * Maximum width of the tooltip in pixels. Defaults to `350`.
   */
  maxWidth?: number;
  /**
   * Optional HTMLElement to render the portal into.
   * Defaults to `document.body`.
   */
  portalRoot?: HTMLElement;
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
  disabled = false,
  showOnlyOnOverflow = false,
  placement = 'top',
  position,
  offset: offsetDistance = 8,
  delay = 0,
  disableFlip = false,
  disableTransitions = false,
  trigger = 'hover',
  sticky = false,
  maxWidth = 350,
  portalRoot,
}: HoverTooltipProps) => {
  const theme = useTheme();
  const [open, setOpen] = useState(false);
  const arrowRef = useRef(null);

  if (position) {
    placement = position;
  }

  // Close the tooltip when disabled changes to true.
  useEffect(() => {
    if (disabled) {
      setOpen(false);
    }
  }, [disabled]);

  const handleOpenChange = (open: boolean, event?: Event) => {
    if (!open) {
      setOpen(false);
      return;
    }

    if (disabled || !tipContent) {
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

    // Check if the target content overflows its own width or its parent.
    const isOverflowing =
      target.scrollWidth > target.offsetWidth ||
      (parent && target.scrollWidth > parent.offsetWidth);
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

  const isClickTrigger = trigger === 'click';
  const isInteractive = isClickTrigger || sticky;

  const { getReferenceProps, getFloatingProps } = useInteractions([
    useHover(context, {
      enabled: !isClickTrigger,
      delay: { open: openDelay, close: closeDelay },
      handleClose: sticky
        ? safePolygon({ buffer: 12, blockPointerEvents: true })
        : null,
    }),
    useClick(context, { enabled: isClickTrigger }),
    useFocus(context, { enabled: !isClickTrigger }),
    useDismiss(context),
    useRole(context, { role: isClickTrigger ? 'dialog' : 'tooltip' }),
  ]);

  // `children` is a single valid React element, as enforced by the ReactElement type.
  const child = Children.only(children);
  const mergedRef = useMemo(
    () => mergeRefs([refs.setReference, child.props.ref]),
    [refs.setReference, child.props.ref]
  );
  const childWithRef = cloneElement(child, {
    ref: mergedRef,
    ...getReferenceProps(),
  });

  return (
    <>
      {childWithRef}
      {isMounted && tipContent && (
        <FloatingPortal root={portalRoot}>
          <StyledTooltip
            $interactive={isInteractive}
            $maxWidth={maxWidth}
            data-testid="tooltip"
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

const StyledTooltip = styled.div<{
  $interactive?: boolean;
  $maxWidth: number;
}>`
  max-width: ${p => p.$maxWidth}px;
  word-wrap: break-word;
  z-index: 1500;
  border-radius: 4px;
  pointer-events: ${p => (p.$interactive ? 'auto' : 'none')};
  filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.15));
  backdrop-filter: blur(2px);
  a {
    color: ${props => props.theme.colors.tooltip.inverseLinkDefault};
  }
`;

const StyledContent = styled(Text)`
  word-wrap: break-word;
`;
