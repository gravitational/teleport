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

import {
  typography,
  fontSize,
  space,
  color,
  textAlign,
  fontWeight,
} from 'design/system';

const Text = styled.div`
  overflow: hidden;
  text-overflow: ellipsis;
  ${typography}
  ${fontSize}
  ${space}
  ${color}
  ${textAlign}
  ${fontWeight}
`;

Text.displayName = 'Text';

Text.propTypes = {
  ...space.propTypes,
  ...fontSize.propTypes,
  ...textAlign.propTypes,
  ...typography.propTypes,
};

Text.defaultProps = {
  m: 0,
};

export default Text;

/**
 * H1 heading. Example usage: page titles and empty result set notifications.
 *
 * Do not use where `h1` typography is used only to make the text bigger (i.e.
 * there's no following content that is logically tied to the heading).
 */
export const H1 = props => <Text as="h1" typography="newH1" {...props} />;

/** Subtitle for heading level 1. Renders as a paragraph. */
export const Subtitle1 = props => (
  <Text as="p" typography="newSubtitle1" {...props} />
);

/**
 * H2 heading. Example usage: dialog titles, dialog-like side panel titles.
 *
 * Do not use where `h2` typography is used only to make the text bigger (i.e.
 * there's no following content that is logically tied to the heading).
 */
export const H2 = props => <Text as="h2" typography="newH2" {...props} />;

/** Subtitle for heading level 2. Renders as a paragraph. */
export const Subtitle2 = props => (
  <Text as="p" typography="newSubtitle2" {...props} />
);

/**
 * H3 heading. Example usage: explanatory side panel titles, resource enrollment
 * step boxes.
 *
 * Do not use where `h3` typography is used only to make the text stand out more
 * (i.e.  there's no following content that is logically tied to the heading).
 */
export const H3 = props => <Text as="h3" typography="newH3" {...props} />;

/** Subtitle for heading level 3. Renders as a paragraph. */
export const Subtitle3 = props => (
  <Text as="p" typography="newSubtitle3" {...props} />
);

/**
 * H4 heading.
 *
 * Do not use where `h4` typography is used only to make the text stand out more
 * (i.e.  there's no following content that is logically tied to the heading).
 */
export const H4 = props => <Text as="h4" typography="newH4" {...props} />;

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
export const P1 = props => <Text as={P} typography="newBody1" {...props} />;

/**
 * A {@link P} that uses `body2` typography. Applies inter-paragraph spacing if
 * grouped with other paragraphs.
 */
export const P2 = props => <Text as={P} typography="newBody2" {...props} />;

/**
 * A {@link P} that uses `body3` typography. Applies inter-paragraph spacing if
 * grouped with other paragraphs.
 */
export const P3 = props => <Text as={P} typography="newBody3" {...props} />;
