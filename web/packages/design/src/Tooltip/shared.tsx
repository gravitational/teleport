/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
  FloatingContext,
  FloatingPortal,
  offset,
  Placement,
  ReferenceType,
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
import React, { useMemo, useRef, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import Text from 'design/Text';

type TooltipTrigger = 'hover' | 'click';

type UseTooltipProps = {
  /**
   * Specifies the position of tooltip relative to trigger content.
   */
  placement?: Placement;
  /**
   * Offset the tooltip relative to trigger content. Defaults to `8`.
   */
  offset?: number;
  /**
   * Show arrow on the tooltip.
   */
  arrow?: boolean;
  /**
   * Delay opening and/or closing of the tooltip.
   */
  delay?: number | { open: number; close: number };
  /**
   * Flip the tooltip's placement when tooltip runs out of the viewport.
   */
  flip?: boolean;
  /**
   * Transition the tooltip in/out on mount/unmount.
   */
  animate?: boolean;
  /**
   * Trigger for showing the tooltip: hover or click.
   */
  trigger?: TooltipTrigger;
  /**
   * Allow mouse interaction with the tooltip content, and prevent closing on mouseOut.
   */
  interactive?: boolean;
  /**
   * Only show tooltip if trigger content is overflowing its container.
   */
  onlyOnOverflow?: boolean;
  /**
   * Optional callback when tooltip open state changes.
   */
  onOpenChange?: (open: boolean) => void;
};

type UseTooltipReturn = {
  context: FloatingContext;
  isMounted: boolean;
  refs: {
    arrow: React.RefObject<SVGSVGElement>;
    reference: React.MutableRefObject<ReferenceType | null>;
    floating: React.RefObject<HTMLElement | null>;
  };
  props: {
    arrow?: {
      ref: React.Ref<SVGSVGElement>;
      context: FloatingContext;
      style: React.CSSProperties;
    } & Record<string, unknown>;
    reference: {
      ref: (node: ReferenceType | null) => void;
    } & Record<string, unknown>;
    floating: {
      ref: (node: HTMLElement | null) => void;
      style: React.CSSProperties;
    } & Record<string, unknown>;
  };
};

const ARROW_HEIGHT = 7;
const ANIMATE_DURATION = 100;

export const useTooltip = ({
  placement = 'top',
  trigger = 'hover',
  interactive = false,
  offset: offsetDistance = 8,
  delay = 0,
  arrow: useArrow = false,
  flip: useFlip = true,
  animate = true,
  onlyOnOverflow = false,
  onOpenChange,
}: UseTooltipProps = {}): UseTooltipReturn => {
  const theme = useTheme();
  const [open, setOpen] = useState(false);
  const arrowRef = useRef<SVGSVGElement | null>(null);

  const setOpenState = (value: boolean) => {
    setOpen(value);
    if (onOpenChange) {
      onOpenChange(value);
    }
  };

  const middleware = [
    offset(offsetDistance + (useArrow ? ARROW_HEIGHT : 0)),
    useFlip && flip(),
    shift({ padding: 8 }),
    useArrow && arrow({ element: arrowRef }),
  ].filter(Boolean);

  const { x, y, strategy, refs, context } = useFloating({
    placement,
    open,
    onOpenChange: setOpenState,
    middleware,
    whileElementsMounted: autoUpdate,
  });

  const { isMounted, styles: transitionStyles } = useTransitionStyles(context, {
    duration: ANIMATE_DURATION,
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

  const interactions = [
    useClick(context, { enabled: trigger === 'click' }),
    useHover(context, {
      enabled: trigger === 'hover',
      delay: { open: openDelay, close: closeDelay },
      handleClose: interactive ? safePolygon() : null,
      restMs: interactive ? 150 : undefined,
    }),
    useFocus(context),
    useDismiss(context),
    useRole(context, { role: 'tooltip' }),
  ];

  const { getReferenceProps, getFloatingProps } = useInteractions(interactions);

  const handleMouseEnter = (event: React.MouseEvent<Element>) => {
    if (onlyOnOverflow) {
      const { currentTarget } = event;
      if (
        currentTarget instanceof Element &&
        currentTarget.parentElement &&
        currentTarget.scrollWidth > currentTarget.parentElement.offsetWidth
      ) {
        setOpenState(true);
      }
    } else if (trigger === 'hover') {
      setOpenState(true);
    }
  };

  const handleMouseLeave = () => {
    if (interactive) return;
    if (trigger === 'hover') {
      setOpenState(false);
    }
  };

  const handleClick = () => {
    if (trigger === 'click') {
      setOpenState(!open);
    }
  };

  const getRefProps = (additionalProps = {}) => {
    const props: any = { ...additionalProps };

    if (trigger === 'hover') {
      props.onMouseEnter = handleMouseEnter;
      props.onMouseLeave = handleMouseLeave;
    } else if (trigger === 'click') {
      props.onClick = handleClick;
    }

    return getReferenceProps(props);
  };

  const pointerEvents: 'auto' | 'none' =
    trigger === 'hover' && interactive ? 'auto' : 'none';

  return {
    context,
    isMounted,
    refs: {
      arrow: arrowRef,
      reference: refs.reference,
      floating: refs.floating,
    },
    props: {
      arrow: useArrow
        ? {
            ref: arrowRef,
            context,
            height: ARROW_HEIGHT,
            style: {
              fill: theme.colors.tooltip.background,
              pointerEvents: 'none' as const,
            },
          }
        : undefined,
      reference: {
        ref: refs.setReference,
        ...getRefProps(),
      },
      floating: {
        ref: refs.setFloating,
        style: {
          position: strategy,
          top: y ?? 0,
          left: x ?? 0,
          background: theme.colors.tooltip.background,
          backdropFilter: 'blur(2px)',
          color: theme.colors.text.primaryInverse,
          pointerEvents,
          ...(animate ? transitionStyles : { opacity: 1 }),
        },
        ...getFloatingProps(),
      },
    },
  };
};

export const BaseTooltip = ({
  children,
  content,
  maxWidth,
  testId = 'tooltip',
  contentTestId = 'tooltip-msg',
  ...tooltipProps
}: {
  children: React.ReactElement;
  content: React.ReactNode;
  maxWidth?: number;
  testId?: string;
  contentTestId?: string;
} & UseTooltipProps) => {
  const { isMounted, props } = useTooltip(tooltipProps);
  const childrenWithProps = useMemo(
    () =>
      React.cloneElement(children, {
        ['data-testid']: testId,
        ...props.reference,
        ...children.props,
      }),
    [children, props.reference, testId]
  );

  return (
    <>
      {childrenWithProps}
      {isMounted && (
        <FloatingPortal>
          <StyledTooltip maxWidth={maxWidth} {...props.floating}>
            {props.arrow && <FloatingArrow {...props.arrow} />}
            <StyledTooltipContent
              px={3}
              py={2}
              maxWidth={maxWidth}
              data-testid={contentTestId}
            >
              {content}
            </StyledTooltipContent>
          </StyledTooltip>
        </FloatingPortal>
      )}
    </>
  );
};

const StyledTooltip = styled.div<{ maxWidth?: number }>`
  max-width: ${({ maxWidth }) => maxWidth || 350}px;
  word-wrap: break-word;
  z-index: 1500;
  border-radius: 4px;
  filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.15));
  backdrop-filter: blur(2px);
`;

const StyledTooltipContent = styled(Text)<{ maxWidth?: number }>`
  max-width: ${({ maxWidth }) => maxWidth || 350}px;
  word-wrap: break-word;
`;
