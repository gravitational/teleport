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

import { Flex, Text } from 'design';

import { Tag } from './Tag';

export default {
  title: 'Design/Tag',
};

export const Variants = () => (
  <Flex flexDirection="column" gap={4} p={4} bg="levels.surface">
    <Text typography="body2" bold>
      Subtle (default)
    </Text>
    <Flex gap={2} flexWrap="wrap">
      <Tag>env: production</Tag>
      <Tag>region: us-east-1</Tag>
      <Tag>team: platform</Tag>
    </Flex>

    <Text typography="body2" bold>
      Outline
    </Text>
    <Flex gap={2} flexWrap="wrap">
      <Tag variant="outline">env: staging</Tag>
      <Tag variant="outline">region: eu-west-1</Tag>
      <Tag variant="outline">team: security</Tag>
    </Flex>

    <Text typography="body2" bold>
      Dismissible
    </Text>
    <Flex gap={2} flexWrap="wrap">
      <Tag onDismiss={() => {}}>removable-tag</Tag>
      <Tag variant="outline" onDismiss={() => {}}>
        also-removable
      </Tag>
    </Flex>

    <Text typography="body2" bold>
      Interactive (hover to see effect)
    </Text>
    <Flex gap={2} flexWrap="wrap">
      <Tag onClick={() => {}}>clickable-tag</Tag>
      <Tag variant="outline" onClick={() => {}}>
        clickable-outline
      </Tag>
    </Flex>
  </Flex>
);
