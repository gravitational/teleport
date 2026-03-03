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

import { type ComponentType, type HTMLAttributes, type ReactNode } from 'react';
import styled, { css, useTheme } from 'styled-components';

import { CircleCheck, CircleCross, Info, Question, Warning } from 'design/Icon';
import type { IconProps } from 'design/Icon/Icon';
import { pillBase } from 'design/pillStyles';
import { space, type SpaceProps } from 'design/system';
import type { Theme } from 'design/theme';

export type StatusKind =
  | 'success'
  | 'warning'
  | 'info'
  | 'danger'
  | 'neutral'
  | 'primary';

export type StatusVariant = 'filled' | 'filled-tonal' | 'border';

interface KindColors {
  solid: string;
  tonal: string;
  accent: string;
}

function getKindColors(theme: Theme, kind: StatusKind): KindColors {
  const { interactive, dataVisualisation } = theme.colors;

  switch (kind) {
    case 'success':
      return {
        solid: interactive.solid.success.default,
        tonal: interactive.tonal.success[1],
        accent: dataVisualisation.tertiary.caribbean,
      };
    case 'warning':
      return {
        solid: interactive.solid.alert.default,
        tonal: interactive.tonal.alert[2],
        accent: dataVisualisation.tertiary.sunflower,
      };
    case 'info':
      return {
        solid: interactive.solid.accent.default,
        tonal: interactive.tonal.informational[2],
        accent: dataVisualisation.tertiary.picton,
      };
    case 'danger':
      return {
        solid: interactive.solid.danger.default,
        tonal: interactive.tonal.danger[2],
        accent: interactive.solid.danger.active,
      };
    case 'neutral':
      return {
        solid: theme.colors.text.muted,
        tonal: interactive.tonal.neutral[1],
        accent: theme.colors.text.slightlyMuted,
      };
    case 'primary':
      return {
        solid: interactive.solid.primary.default,
        tonal: interactive.tonal.primary[0],
        accent: dataVisualisation.tertiary.purple,
      };
  }
}

const defaultIcons: Record<StatusKind, ComponentType<IconProps>> = {
  success: CircleCheck,
  warning: Warning,
  info: Info,
  danger: CircleCross,
  neutral: Question,
  primary: CircleCheck,
};

function getIconColor(
  colors: KindColors,
  variant: StatusVariant,
  theme: Theme
): string {
  if (variant === 'filled') {
    return theme.colors.text.primaryInverse;
  }
  return colors.accent;
}

interface StyledStatusProps extends SpaceProps {
  $kind: StatusKind;
  $variant: StatusVariant;
}

function variantStyles({
  $kind,
  $variant,
  theme,
}: StyledStatusProps & { theme: Theme }) {
  const c = getKindColors(theme, $kind);

  switch ($variant) {
    case 'filled':
      return css`
        background: ${c.solid};
        color: ${theme.colors.text.primaryInverse};
        border: 1px solid transparent;
      `;
    case 'filled-tonal':
      return css`
        background: ${c.tonal};
        color: ${theme.colors.text.main};
        border: 1px solid ${c.accent};
      `;
    case 'border':
      return css`
        background: transparent;
        color: ${theme.colors.text.main};
        border: 1px solid ${c.accent};
      `;
  }
}

const StyledStatus = styled.span<StyledStatusProps>`
  ${pillBase}
  cursor: default;
  ${variantStyles}
  ${space}
`;

export interface StatusProps
  extends SpaceProps, Omit<HTMLAttributes<HTMLSpanElement>, 'color'> {
  kind: StatusKind;
  variant?: StatusVariant;
  // Provide a custom icon to override the default
  icon?: ComponentType<IconProps>;
  // Render the badge without an icon
  noIcon?: boolean;
  children: ReactNode;
}

export function Status({
  kind,
  variant = 'filled-tonal',
  icon,
  noIcon,
  children,
  ...rest
}: StatusProps) {
  const theme = useTheme() as Theme;

  let iconElement: ReactNode = null;
  if (!noIcon) {
    const IconComponent = icon ?? defaultIcons[kind];
    const colors = getKindColors(theme, kind);
    iconElement = (
      <IconComponent
        size="small"
        color={getIconColor(colors, variant, theme)}
      />
    );
  }

  return (
    <StyledStatus $kind={kind} $variant={variant} {...rest}>
      {iconElement}
      {children}
    </StyledStatus>
  );
}
