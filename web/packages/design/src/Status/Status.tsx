/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { type ComponentType, type ReactNode } from 'react';
import styled, { css, useTheme } from 'styled-components';

import { CircleCheck, CircleCross, Info, Question, Warning } from 'design/Icon';
import type { IconProps } from 'design/Icon/Icon';
import { pillBase } from 'design/pillStyles';
import { space, type SpaceProps } from 'design/system';
import type { Theme } from 'design/theme';

import { getVariantColors } from './statusColors';

export type StatusKind =
  | 'success'
  | 'warning'
  | 'info'
  | 'danger'
  | 'neutral'
  | 'primary';

export type StatusVariant =
  // Primary actions, critical states (APPROVED/DENIED badges).
  | 'filled'
  // General status indicators (default).
  | 'filled-tonal'
  // Secondary context alongside other UI (version pills, health dots).
  | 'border'
  // Background metadata that shouldn't compete for attention.
  | 'filled-subtle';

const defaultIcons: Record<StatusKind, ComponentType<IconProps>> = {
  success: CircleCheck,
  warning: Warning,
  info: Info,
  danger: CircleCross,
  neutral: Question,
  primary: CircleCheck,
};

interface StyledStatusProps extends SpaceProps {
  $kind: StatusKind;
  $variant: StatusVariant;
}

function variantStyles({
  $kind,
  $variant,
  theme,
}: StyledStatusProps & { theme: Theme }) {
  const c = getVariantColors(theme, $kind, $variant);
  return css`
    background: ${c.bg};
    color: ${c.fg};
    border: 1px solid ${c.border};
  `;
}

const StyledStatus = styled.span<StyledStatusProps>`
  ${pillBase}
  cursor: default;
  ${variantStyles}
  ${space}
`;

function renderIcon(
  icon: StatusProps['icon'],
  kind: StatusKind,
  color: string
): ReactNode {
  if (icon === false) return null;
  const Icon = icon ?? defaultIcons[kind];
  return <Icon size="small" color={color} />;
}

export interface StatusProps extends SpaceProps {
  kind: StatusKind;
  variant?: StatusVariant;
  // Custom icon component reference, or `false` to hide the icon entirely.
  icon?: ComponentType<IconProps> | false;
  children: ReactNode;
}

export function Status({
  kind,
  variant = 'filled-tonal',
  icon,
  children,
  ...rest
}: StatusProps) {
  const theme = useTheme() as Theme;
  const v = getVariantColors(theme, kind, variant);

  return (
    <StyledStatus role="status" $kind={kind} $variant={variant} {...rest}>
      {renderIcon(icon, kind, v.iconColor)}
      {children}
    </StyledStatus>
  );
}
