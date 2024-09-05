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

import PropTypes from 'prop-types';

import { ResponsiveValue } from 'styled-system';

import { SharedStyles, Theme } from 'design/theme/themes/types';

export interface TypographyProps {
  caps?: boolean;
  bold?: boolean;
  italic?: boolean;
  mono?: boolean;
  breakAll?: boolean;
  typography?: ResponsiveValue<keyof SharedStyles['typography']>;
}

interface TypographyPropsWithTheme extends TypographyProps {
  theme: Theme;
}

function getTypography(props: TypographyPropsWithTheme) {
  const { typography, theme } = props;
  return {
    ...theme.typography[typography],
    ...caps(props),
    ...breakAll(props),
    ...bold(props),
    ...mono(props),
  };
}

function caps(props: TypographyProps) {
  return props.caps ? { textTransform: 'uppercase' } : null;
}

function mono(props: TypographyPropsWithTheme) {
  return props.mono ? { fontFamily: props.theme.fonts.mono } : null;
}

function breakAll(props: TypographyProps) {
  return props.breakAll ? { wordBreak: 'break-all' } : null;
}

function bold(props: TypographyPropsWithTheme) {
  return props.bold ? { fontWeight: props.theme.fontWeights.bold } : null;
}

getTypography.propTypes = {
  caps: PropTypes.bool,
  bold: PropTypes.bool,
  italic: PropTypes.bool,
  color: PropTypes.string,
};

export default getTypography;
