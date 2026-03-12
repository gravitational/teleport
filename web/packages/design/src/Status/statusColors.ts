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

import type { Theme } from 'design/theme';

import type { StatusKind, StatusVariant } from './Status';

export interface KindColors {
  solid: string;
  tonal: string;
  accent: string;
}

export function getKindColors(theme: Theme, kind: StatusKind): KindColors {
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

export interface VariantStyles {
  bg: string;
  fg: string;
  border: string;
  iconColor: string;
}

export function getVariantColors(
  theme: Theme,
  kind: StatusKind,
  variant: StatusVariant
): VariantStyles {
  const c = getKindColors(theme, kind);
  const inverse = theme.colors.text.primaryInverse;
  const main = theme.colors.text.main;

  switch (variant) {
    case 'filled':
      return {
        bg: c.solid,
        fg: inverse,
        border: 'transparent',
        iconColor: inverse,
      };
    case 'filled-tonal':
      return { bg: c.tonal, fg: main, border: c.accent, iconColor: c.accent };
    case 'filled-subtle':
      return {
        bg: c.tonal,
        fg: main,
        border: 'transparent',
        iconColor: c.accent,
      };
    case 'border':
      return {
        bg: 'transparent',
        fg: main,
        border: c.accent,
        iconColor: c.accent,
      };
  }
}
