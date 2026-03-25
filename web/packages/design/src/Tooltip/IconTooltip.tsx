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

import React, { PropsWithChildren } from 'react';
import styled from 'styled-components';

import * as Icons from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip/HoverTooltip';

type ToolTipKind = 'info' | 'warning' | 'error';

export const IconTooltip: React.FC<
  PropsWithChildren<
    {
      muteIconColor?: boolean;
      kind?: ToolTipKind;
    } & Omit<React.ComponentProps<typeof HoverTooltip>, 'children'>
  >
> = ({
  children,
  muteIconColor = false,
  kind = 'info',
  ...hoverTooltipProps
}) => {
  return (
    <HoverTooltip {...hoverTooltipProps} tipContent={children}>
      <IconWrapper>
        <ToolTipIcon kind={kind} muteIconColor={muteIconColor} />
      </IconWrapper>
    </HoverTooltip>
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

const IconWrapper = styled.span`
  display: inline-flex;
  vertical-align: middle;
  cursor: pointer;
  height: 18px;
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
