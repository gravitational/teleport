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
import styled from 'styled-components';
import { Popover, Flex, Text } from 'design';
import { JustifyContentProps } from 'design/system';

type OriginProps = {
  vertical: string;
  horizontal: string;
};

export const HoverTooltip: React.FC<
  PropsWithChildren<{
    tipContent: string | undefined;
    showOnlyOnOverflow?: boolean;
    className?: string;
    anchorOrigin?: OriginProps;
    transformOrigin?: OriginProps;
    justifyContentProps?: JustifyContentProps;
  }>
> = ({
  tipContent,
  children,
  showOnlyOnOverflow = false,
  className,
  anchorOrigin = { vertical: 'top', horizontal: 'center' },
  transformOrigin = { vertical: 'bottom', horizontal: 'center' },
  justifyContentProps = {},
}) => {
  const [anchorEl, setAnchorEl] = useState<Element | undefined>();
  const open = Boolean(anchorEl);

  function handlePopoverOpen(event: React.MouseEvent<Element>) {
    const { target } = event;

    if (showOnlyOnOverflow) {
      // Calculate whether the content is overflowing the parent in order to determine
      // whether we want to show the tooltip.
      if (
        target instanceof Element &&
        target.scrollWidth > target.parentElement.offsetWidth
      ) {
        setAnchorEl(event.currentTarget);
      }
      return;
    }

    setAnchorEl(event.currentTarget);
  }

  function handlePopoverClose() {
    setAnchorEl(null);
  }

  // Don't render the tooltip if the content is undefined.
  if (!tipContent) {
    return <>{children}</>;
  }

  return (
    <Flex
      aria-owns={open ? 'mouse-over-popover' : undefined}
      onMouseEnter={handlePopoverOpen}
      onMouseLeave={handlePopoverClose}
      className={className}
      {...justifyContentProps}
    >
      {children}
      <Popover
        modalCss={modalCss}
        onClose={handlePopoverClose}
        open={open}
        anchorEl={anchorEl}
        anchorOrigin={anchorOrigin}
        transformOrigin={transformOrigin}
        disableRestoreFocus
      >
        <StyledOnHover
          px={2}
          py={1}
          fontWeight="regular"
          typography="subtitle2"
          css={`
            word-wrap: break-word;
          `}
        >
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
  color: ${props => props.theme.colors.text.main};
  background-color: ${props => props.theme.colors.tooltip.background};
  max-width: 350px;
`;
