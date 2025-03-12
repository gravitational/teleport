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

import React, { PropsWithChildren, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import Flex from 'design/Flex';
import Popover, { Origin } from 'design/Popover';
import { Position } from 'design/Popover/Popover';
import { FlexBasisProps, JustifyContentProps } from 'design/system';
import Text from 'design/Text';

import { anchorOriginForPosition, transformOriginForPosition } from './shared';

export const HoverTooltip: React.FC<
  PropsWithChildren<{
    tipContent?: React.ReactNode;
    showOnlyOnOverflow?: boolean;
    className?: string;
    /**
     * Specifies the position of tooltip relative to content. Used if neither
     * anchor or transform origins are specified.
     */
    position?: Position;
    anchorOrigin?: Origin;
    transformOrigin?: Origin;
    justifyContentProps?: JustifyContentProps;
    flexBasisProps?: FlexBasisProps;
    sticky?: boolean;
    trigger?: 'click' | 'hover';
  }>
> = ({
  tipContent,
  children,
  showOnlyOnOverflow = false,
  className,
  position = 'top',
  anchorOrigin,
  transformOrigin,
  justifyContentProps = {},
  flexBasisProps = {},
  sticky = false,
  trigger = 'hover',
}) => {
  const theme = useTheme();
  const [anchorEl, setAnchorEl] = useState<Element | undefined>();
  const open = Boolean(anchorEl);

  function handlePopoverOpen(event: React.MouseEvent<Element>) {
    const { target } = event;

    if (showOnlyOnOverflow) {
      // Calculate whether the content is overflowing the parent in order to determine
      // whether we want to show the tooltip.
      if (
        target instanceof Element &&
        target.parentElement &&
        target.scrollWidth > target.parentElement.offsetWidth
      ) {
        setAnchorEl(event.currentTarget);
      }
      return;
    }

    setAnchorEl(event.currentTarget);
  }

  function handlePopoverClose() {
    setAnchorEl(undefined);
  }

  const triggerOnHoverProps = {
    onMouseEnter: handlePopoverOpen,
    onMouseLeave: sticky ? undefined : handlePopoverClose,
  };
  const triggerOnClickProps = {
    onClick: handlePopoverOpen,
  };

  // Don't render the tooltip if the content is undefined.
  if (!tipContent) {
    return <>{children}</>;
  }

  if (!transformOrigin && !anchorOrigin) {
    transformOrigin = transformOriginForPosition(position);
    anchorOrigin = anchorOriginForPosition(position);
  } else {
    if (!anchorOrigin) {
      anchorOrigin = { vertical: 'top', horizontal: 'center' };
    }
    if (!transformOrigin) {
      transformOrigin = { vertical: 'bottom', horizontal: 'center' };
    }
  }

  return (
    <Flex
      aria-owns={open ? 'mouse-over-popover' : undefined}
      {...(trigger === 'hover' && triggerOnHoverProps)}
      {...(trigger === 'click' && triggerOnClickProps)}
      className={className}
      {...justifyContentProps}
      {...flexBasisProps}
    >
      {children}
      <Popover
        modalCss={() =>
          trigger === 'hover' && `pointer-events: ${sticky ? 'auto' : 'none'}`
        }
        popoverCss={() => ({
          background: theme.colors.tooltip.background,
          backdropFilter: 'blur(2px)',
        })}
        onClose={handlePopoverClose}
        open={open}
        anchorEl={anchorEl}
        anchorOrigin={anchorOrigin}
        transformOrigin={transformOrigin}
        arrow
        popoverMargin={4}
        disableRestoreFocus
      >
        <StyledOnHover px={3} py={2}>
          {tipContent}
        </StyledOnHover>
      </Popover>
    </Flex>
  );
};

const modalCss = () => `
  pointer-events: none;
`;

const StyledOnHover = styled(Text)`
  color: ${props => props.theme.colors.text.primaryInverse};
  max-width: 350px;
  word-wrap: break-word;
`;
