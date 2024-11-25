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
import styled from 'styled-components';
import { ResponsiveValue } from 'styled-system';

import {
  color,
  ColorProps,
  fontSize,
  FontSizeProps,
  fontWeight,
  space,
  SpaceProps,
  textAlign,
  TextAlignProps,
  typography,
  TypographyProps,
} from 'design/system';
import { shouldForwardTypographyProp } from 'design/system/typography';
import { fontWeights } from 'design/theme/typography';

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

/** Subtitle for heading level 1. Renders as a paragraph. */
export const Subtitle1 = (props: TextProps) => (
  <Text as="p" typography="subtitle1" {...props} />
);

/**
 * H2 heading. Example usage: dialog titles, dialog-like side panel titles.
 *
 * Do not use where `h2` typography is used only to make the text bigger (i.e.
 * there's no following content that is logically tied to the heading).
 */
export const H2 = (props: TextProps) => (
  <Text as="h2" typography="h2" {...props} />
);

/** Subtitle for heading level 2. Renders as a paragraph. */
export const Subtitle2 = (props: TextProps) => (
  <Text as="p" typography="subtitle2" {...props} />
);

/**
 * H3 heading. Example usage: explanatory side panel titles, resource enrollment
 * step boxes.
 *
 * Do not use where `h3` typography is used only to make the text stand out more
 * (i.e.  there's no following content that is logically tied to the heading).
 */
export const H3 = (props: TextProps) => (
  <Text as="h3" typography="h3" {...props} />
);

/** Subtitle for heading level 3. Renders as a paragraph. */
export const Subtitle3 = (props: TextProps) => (
  <Text as="p" typography="subtitle3" {...props} />
);

/**
 * H4 heading.
 *
 * Do not use where `h4` typography is used only to make the text stand out more
 * (i.e.  there's no following content that is logically tied to the heading).
 */
export const H4 = (props: TextProps) => (
  <Text as="h4" typography="h4" {...props} />
);

/**
 * A paragraph. Use for text consisting of actual sentences. Applies
 * inter-paragraph spacing if grouped with other paragraphs, but doesn't apply
 * typography. Use directly when typography is expected to be set by the parent
 * component; prefer {@link P1}, {@link P2}, {@link P3} otherwise.
 */
export const P = styled(Text).attrs({ as: 'p' })`
  p + & {
    margin-top: ${props => props.theme.space[3]}px;
    // Allow overriding.
    ${space}
  }
`;

/**
 * A {@link P} that uses `body1` typography. Applies inter-paragraph spacing if
 * grouped with other paragraphs.
 */
export const P1 = (props: TextProps) => (
  <Text as={P} typography="body1" {...props} />
);

/**
 * A {@link P} that uses `body2` typography. Applies inter-paragraph spacing if
 * grouped with other paragraphs.
 */
export const P2 = (props: TextProps) => (
  <Text as={P} typography="body2" {...props} />
);

/**
 * A {@link P} that uses `body3` typography. Applies inter-paragraph spacing if
 * grouped with other paragraphs.
 */
export const P3 = (props: TextProps) => (
  <Text as={P} typography="body3" {...props} />
);
