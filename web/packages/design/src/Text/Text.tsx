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

import styled from 'styled-components';

import { ResponsiveValue } from 'styled-system';

import { Property } from 'csstype';

import {
  typography,
  TypographyProps,
  fontSize,
  FontSizeProps,
  space,
  SpaceProps,
  color,
  ColorProps,
  textAlign,
  TextAlignProps,
  fontWeight,
} from 'design/system';
import { fontWeights } from 'design/theme/typography';
import { shouldForwardTypographyProp } from 'design/system/typography';

interface FontWeightProps {
  fontWeight?: ResponsiveValue<Property.FontWeight | keyof typeof fontWeights>;
}

export type TextProps<E extends React.ElementType = 'div'> =
  React.ComponentPropsWithoutRef<E> &
    TypographyProps &
    FontSizeProps &
    SpaceProps &
    ColorProps &
    TextAlignProps &
    FontWeightProps;

const Text = styled.div.withConfig({
  shouldForwardProp: shouldForwardTypographyProp,
})<TextProps>`
  overflow: hidden;
  text-overflow: ellipsis;
  margin: 0;
  ${typography}
  ${fontSize}
  ${space}
  ${color}
  ${textAlign}
  ${fontWeight}
`;

Text.displayName = 'Text';

export default Text;

/**
 * H1 heading. Example usage: page titles and empty result set notifications.
 *
 * Do not use where `h1` typography is used only to make the text bigger (i.e.
 * there's no following content that is logically tied to the heading).
 */
export const H1 = (props: TextProps) => (
  <Text as="h1" typography="h1" {...props} />
);

/**
 * H2 heading. Example usage: side panel titles, dialog titles.
 *
 * Do not use where `h1` typography is used only to make the text bigger (i.e.
 * there's no following content that is logically tied to the heading).
 */
export const H2 = (props: TextProps) => (
  <Text as="h2" typography="h2" {...props} />
);
