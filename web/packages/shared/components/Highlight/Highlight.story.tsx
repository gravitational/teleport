/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
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
