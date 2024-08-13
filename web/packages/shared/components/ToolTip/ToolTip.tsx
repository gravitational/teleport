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

import { Popover, Text } from 'design';
import * as Icons from 'design/Icon';

export const ToolTipInfo: React.FC<
  PropsWithChildren<{
    trigger?: 'click' | 'hover';
    muteIconColor?: boolean;
    sticky?: boolean;
    maxWidth?: number;
    kind?: 'info' | 'warning' | 'error';
  }>
> = ({
  children,
  trigger = 'hover',
  muteIconColor,
  sticky = false,
  maxWidth = 350,
  kind = 'info',
}) => {
  const [anchorEl, setAnchorEl] = useState();
  const open = Boolean(anchorEl);

  function handlePopoverOpen(event) {
    setAnchorEl(event.currentTarget);
  }

  function handlePopoverClose() {
    setAnchorEl(null);
  }

  const triggerOnHoverProps = {
    onMouseEnter: handlePopoverOpen,
    onMouseLeave: sticky ? undefined : handlePopoverClose,
  };
  const triggerOnClickProps = {
    onClick: handlePopoverOpen,
  };

  return (
    <>
      <span
        role="icon"
        aria-owns={open ? 'mouse-over-popover' : undefined}
        {...(trigger === 'hover' && triggerOnHoverProps)}
        {...(trigger === 'click' && triggerOnClickProps)}
        css={`
          &:hover {
            cursor: pointer;
          }
          vertical-align: middle;
          display: inline-block;
          height: 18px;
        `}
      >
        {kind === 'info' && (
          <InfoIcon $muteIconColor={muteIconColor} size="medium" />
        )}
        {kind === 'warning' && (
          <WarningIcon $muteIconColor={muteIconColor} size="medium" />
        )}
        {kind === 'error' && (
          <ErrorIcon $muteIconColor={muteIconColor} size="medium" />
        )}
      </span>
      <Popover
        modalCss={() =>
          trigger === 'hover' && `pointer-events: ${sticky ? 'auto' : 'none'}`
        }
        onClose={handlePopoverClose}
        open={open}
        anchorEl={anchorEl}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'left',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'left',
        }}
      >
        <StyledOnHover px={3} py={2} $maxWidth={maxWidth}>
          {children}
        </StyledOnHover>
      </Popover>
    </>
  );
};

const StyledOnHover = styled(Text)<{ $maxWidth: number }>`
  color: ${props => props.theme.colors.text.main};
  background-color: ${props => props.theme.colors.tooltip.background};
  max-width: ${p => p.$maxWidth}px;
`;

const InfoIcon = styled(Icons.Info)<{ $muteIconColor?: boolean }>`
  height: 18px;
  width: 18px;
  color: ${p => (p.$muteIconColor ? p.theme.colors.text.disabled : 'inherit')};
`;

const WarningIcon = styled(Icons.Warning)<{ $muteIconColor?: boolean }>`
  height: 18px;
  width: 18px;
  color: ${p =>
    p.$muteIconColor
      ? p.theme.colors.text.disabled
      : p.theme.colors.interactive.solid.alert.default.background};
`;

const ErrorIcon = styled(Icons.Warning)<{ $muteIconColor?: boolean }>`
  height: 18px;
  width: 18px;
  color: ${p =>
    p.$muteIconColor
      ? p.theme.colors.text.disabled
      : p.theme.colors.interactive.solid.danger.default.background};
`;
