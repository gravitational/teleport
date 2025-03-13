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
import React, { PropsWithChildren, useRef, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import Flex from 'design/Flex';
import { FlexBasisProps, JustifyContentProps } from 'design/system';
import Text from 'design/Text';

type HoverTooltipProps = {
  tipContent?: React.ReactNode;
  showOnlyOnOverflow?: boolean;
  className?: string;
  /**
   * Specifies the position of tooltip relative to content. Used if neither
   * anchor nor transform origins are specified.
   */
  placement?: Placement;
  justifyContentProps?: JustifyContentProps;
  flexBasisProps?: FlexBasisProps;
  // Optional additional props for FloatingUI
  offset?: number;
  delay?: number | { open: number; close: number };
  disableFlip?: boolean;
  disableTransitions?: boolean;
};

export const HoverTooltip = ({
  tipContent,
  children,
  showOnlyOnOverflow = false,
  className,
  placement = 'top',
  justifyContentProps = {},
  flexBasisProps = {},
  offset: offsetDistance = 8,
  delay = 0,
  disableFlip = false,
  disableTransitions = false,
}: PropsWithChildren<HoverTooltipProps>) => {
  const theme = useTheme();
  const [open, setOpen] = useState(false);
  const arrowRef = useRef(null);
  const contentRef = useRef<HTMLElement | null>(null);

  const { x, y, strategy, refs, context } = useFloating({
    placement,
    open,
    onOpenChange: setOpen,
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

  const { getReferenceProps, getFloatingProps } = useInteractions([
    useHover(context, {
      delay: { open: openDelay, close: closeDelay },
      handleClose: null,
    }),
    useFocus(context),
    useDismiss(context),
    useRole(context, { role: 'tooltip' }),
  ]);

  if (!tipContent) {
    return <>{children}</>;
  }

  const handleMouseEnter = (event: React.MouseEvent<Element>) => {
    const { currentTarget } = event;
    contentRef.current = currentTarget as HTMLElement;

    if (showOnlyOnOverflow) {
      if (
        currentTarget instanceof Element &&
        currentTarget.parentElement &&
        currentTarget.scrollWidth > currentTarget.parentElement.offsetWidth
      ) {
        setOpen(true);
      }
      return;
    }

    setOpen(true);
  };

  return (
    <Flex
      ref={refs.setReference}
      className={className}
      {...getReferenceProps({
        onMouseEnter: handleMouseEnter,
        onMouseLeave: () => setOpen(false),
      })}
      {...justifyContentProps}
      {...flexBasisProps}
    >
      {children}
      <FloatingPortal>
        {isMounted && (
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
              style={{ fill: theme.colors.tooltip.background }}
            />
            <StyledContent px={3} py={2}>
              {tipContent}
            </StyledContent>
          </StyledTooltip>
        )}
      </FloatingPortal>
    </Flex>
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
