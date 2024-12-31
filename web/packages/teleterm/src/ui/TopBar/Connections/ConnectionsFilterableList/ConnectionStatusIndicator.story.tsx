/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Box, Flex, H2, Text } from 'design';

import { StaticListItem } from 'teleterm/ui/components/ListItem';

import { ConnectionStatusIndicator } from './ConnectionStatusIndicator';

export default {
  title: 'Teleterm/TopBar/ConnectionStatusIndicator',
  decorators: [
    Story => {
      return (
        <Box width={324} bg="levels.elevated">
          <Story />
        </Box>
      );
    },
  ],
};

export const Story = () => (
  <Flex flexDirection="column" gap={3} p={2}>
    <Box>
      <H2>Block</H2>
      <ul
        css={`
          padding: 0;
          margin: 0;
        `}
      >
        <ListItem>
          <ConnectionStatusIndicator status="processing" mr={3} />{' '}
          <Text>{text[1]}</Text>
        </ListItem>
        <ListItem>
          <ConnectionStatusIndicator status="off" mr={3} />{' '}
          <Text>{text[2]}</Text>
        </ListItem>
        <ListItem>
          <ConnectionStatusIndicator status="on" mr={3} />{' '}
          <Text>{text[0]}</Text>
        </ListItem>
        <ListItem>
          <ConnectionStatusIndicator status="warning" mr={3} />{' '}
          <Text>{text[3]}</Text>
        </ListItem>
        <ListItem>
          <ConnectionStatusIndicator status="error" mr={3} />{' '}
          <Text>{text[4]}</Text>
        </ListItem>
      </ul>
    </Box>

    <Box>
      <H2>Inline</H2>
      <Box pl={2}>
        <Text>
          <ConnectionStatusIndicator inline status="processing" mr={3} />{' '}
          {text[1]}
        </Text>
        <Text>
          <ConnectionStatusIndicator inline status="off" mr={3} /> {text[2]}
        </Text>
        <Text>
          <ConnectionStatusIndicator inline status="on" mr={3} /> {text[0]}
        </Text>
        <Text>
          <ConnectionStatusIndicator inline status="warning" mr={3} /> {text[3]}
        </Text>
        <Text>
          <ConnectionStatusIndicator inline status="error" mr={3} /> {text[4]}
        </Text>
      </Box>
    </Box>
  </Flex>
);

const text = [
  'Lorem ipsum',
  'Et ultrices posuere',
  'Dolor sit amet',
  'Ante ipsum primis',
  'Nec porta augue',
];

const ListItem = styled(StaticListItem)`
  padding: ${props => props.theme.space[1]}px ${props => props.theme.space[2]}px;
`;
