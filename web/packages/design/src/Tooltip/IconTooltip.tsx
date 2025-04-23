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

import * as Icons from 'design/Icon';
import Popover from 'design/Popover';
import { Position } from 'design/Popover/Popover';
import Text from 'design/Text';

import { anchorOriginForPosition, transformOriginForPosition } from './shared';

type ToolTipKind = 'info' | 'warning' | 'error';

export const IconTooltip: React.FC<
  PropsWithChildren<{
    trigger?: 'click' | 'hover';
    position?: Position;
    muteIconColor?: boolean;
    sticky?: boolean;
    maxWidth?: number;
    kind?: ToolTipKind;
  }>
> = ({
  children,
  trigger = 'hover',
  position = 'bottom',
  muteIconColor = false,
  sticky = false,
  maxWidth = 350,
  kind = 'info',
}) => {
  const theme = useTheme();
  const [anchorEl, setAnchorEl] = useState<Element>();
  const open = Boolean(anchorEl);

  function handlePopoverOpen(event: React.MouseEvent) {
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
        <ToolTipIcon kind={kind} muteIconColor={muteIconColor} />
      </span>
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
        anchorOrigin={anchorOriginForPosition(position)}
        transformOrigin={transformOriginForPosition(position)}
        arrow
        popoverMargin={4}
      >
        <StyledOnHover px={3} py={2} $maxWidth={maxWidth}>
          {children}
        </StyledOnHover>
      </Popover>
    </>
  );
};

const ToolTipIcon = ({
  kind,
  muteIconColor,
}: {
  kind: ToolTipKind;
  muteIconColor: boolean;
}) => {
  switch (kind) {
    case 'info':
      return <InfoIcon $muteIconColor={muteIconColor} size="medium" />;
    case 'warning':
      return <WarningIcon $muteIconColor={muteIconColor} size="medium" />;
    case 'error':
      return <ErrorIcon $muteIconColor={muteIconColor} size="medium" />;
  }
};

const StyledOnHover = styled(Text)<{ $maxWidth: number }>`
  color: ${props => props.theme.colors.text.primaryInverse};
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
      : p.theme.colors.interactive.solid.alert.default};
`;

const ErrorIcon = styled(Icons.WarningCircle)<{ $muteIconColor?: boolean }>`
  height: 18px;
  width: 18px;
  color: ${p =>
    p.$muteIconColor
      ? p.theme.colors.text.disabled
      : p.theme.colors.interactive.solid.danger.default};
`;
