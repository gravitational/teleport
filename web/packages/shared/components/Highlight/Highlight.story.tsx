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

import styled from 'styled-components';

import { Flex, Text } from 'design';

import { Highlight } from './Highlight';

export default {
  title: 'Shared/Highlight',
};

export const Story = () => {
  const keywords = [
    'Aliquam',
    'olor',
    // Overlapping matches: 'lor' and 'rem' both match 'lorem', so the whole word should get
    // highlighted.
    'lor',
    'rem',
    // https://www.contentful.com/blog/unicode-javascript-and-the-emoji-family/
    // Unfortunately, the library we use for highlighting seems to match only the first emoji of
    // such group, e.g. searching for the emoji of a son won't match a group in which the son is
    // present.
    '👩',
    '👨‍👨‍👦‍👦',
    '🥑',
  ];
  return (
    <Flex
      flexDirection="column"
      gap={6}
      css={`
        max-width: 60ch;
      `}
    >
      <Flex flexDirection="column" gap={2}>
        <Text>
          Highlighting <code>{keywords.join(', ')}</code> in the below text:
        </Text>
        <Text>
          <Highlight text={loremIpsum} keywords={keywords} />
        </Text>
      </Flex>

      <Flex flexDirection="column" gap={2}>
        <Text>Custom highlighting</Text>
        <Text>
          <CustomHighlight>
            <Highlight text={loremIpsum} keywords={keywords} />
          </CustomHighlight>
        </Text>
      </Flex>
    </Flex>
  );
};

const loremIpsum =
  'Lorem ipsum 👩‍👩‍👧‍👦 dolor sit amet, 👨‍👨‍👦‍👦 consectetur adipiscing elit. 🥑 Aliquam vel augue varius, venenatis velit sit amet, aliquam arcu. Morbi dictum mattis ultrices. Nullam ut porta ipsum, porta ornare nibh. Vivamus magna felis, semper sed enim sit amet, varius rhoncus leo. Aenean ornare convallis sem ut accumsan.';

const CustomHighlight = styled.div`
  mark {
    background-color: magenta;
  }
`;
