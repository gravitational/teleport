/*
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

import { Property } from 'csstype';
import { WebTarget } from 'styled-components';

import { SharedStyles, Theme } from 'design/theme/themes/types';
import { shouldForwardProp } from 'design/ThemeProvider';

export interface TypographyProps {
  caps?: boolean;
  bold?: boolean;
  italic?: boolean;
  mono?: boolean;
  breakAll?: boolean;
  typography?: keyof SharedStyles['typography'];
}

interface TypographyPropsWithTheme extends TypographyProps {
  theme: Theme;
}

function getTypography(props: TypographyPropsWithTheme) {
  const { typography, theme } = props;
  return {
    ...(typography ? theme.typography[typography] : undefined),
    ...caps(props),
    ...breakAll(props),
    ...bold(props),
    ...mono(props),
  };
}

const typographyProps: Required<{ [k in keyof TypographyProps]: boolean }> = {
  caps: true,
  bold: true,
  italic: true,
  mono: true,
  breakAll: true,
  typography: true,
};

/**
 * Determines whether a property with a given name should be forwarded down as
 * an attribute to an underlying HTML tag. To be used along with styled-components
 */
export function shouldForwardTypographyProp(
  propName: string,
  target: WebTarget
) {
  return !(propName in typographyProps) && shouldForwardProp(propName, target);
}

function caps(props: TypographyProps): {
  textTransform: Property.TextTransform;
} | null {
  return props.caps ? { textTransform: 'uppercase' as const } : null;
}

function mono(props: TypographyPropsWithTheme) {
  return props.mono ? { fontFamily: props.theme.fonts.mono } : null;
}

function breakAll(
  props: TypographyProps
): { wordBreak: Property.WordBreak } | null {
  return props.breakAll ? { wordBreak: 'break-all' } : null;
}

function bold(props: TypographyPropsWithTheme) {
  return props.bold ? { fontWeight: props.theme.fontWeights.bold } : null;
}

export default getTypography;
