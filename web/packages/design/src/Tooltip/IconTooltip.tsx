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

import { type Placement } from '@floating-ui/react';
import React, { PropsWithChildren } from 'react';
import styled from 'styled-components';

import * as Icons from 'design/Icon';
import { BaseTooltip } from 'design/Tooltip/shared';

type ToolTipKind = 'info' | 'warning' | 'error';

type IconToolTipProps = {
  trigger?: 'click' | 'hover';
  muteIconColor?: boolean;
  sticky?: boolean;
  maxWidth?: number;
  kind?: ToolTipKind;
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
   * Flip the tooltip's placement when tooltip runs out of the viewport.
   */
  flip?: boolean;
  /**
   * Transition the tooltip in/out on mount/unmount.
   */
  animate?: boolean;
};

export const IconTooltip: React.FC<PropsWithChildren<IconToolTipProps>> = ({
  children,
  position,
  placement = 'bottom',
  muteIconColor = false,
  kind = 'info',
  trigger = 'hover',
  sticky = false,
  ...tooltipProps
}) => (
  <BaseTooltip
    content={children}
    trigger={trigger}
    placement={position || placement}
    interactive={sticky}
    {...tooltipProps}
    arrow
  >
    <span
      role="icon"
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
  </BaseTooltip>
);

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
