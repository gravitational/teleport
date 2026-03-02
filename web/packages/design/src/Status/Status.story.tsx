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

import { Status, StatusKind, StatusVariant } from './Status';

export default {
  title: 'Design/Status',
};

const kinds: StatusKind[] = [
  'success',
  'warning',
  'info',
  'danger',
  'neutral',
  'primary',
];

const variants: StatusVariant[] = ['filled', 'filled-tonal', 'border'];

export const AllVariants = () => (
  <Flex flexDirection="column" gap={6} p={4} bg="levels.surface">
    <Flex gap={6}>
      <Text
        typography="body2"
        bold
        css={{ width: '100px', textAlign: 'right' }}
      >
        {/* empty for alignment */}
      </Text>
      {variants.map(variant => (
        <Text
          key={variant}
          typography="body2"
          bold
          css={{ width: '160px', textAlign: 'center' }}
        >
          {variant}
        </Text>
      ))}
    </Flex>

    {kinds.map(kind => (
      <Flex key={kind} gap={6} alignItems="center">
        <Text
          typography="body2"
          color="text.slightlyMuted"
          css={{ width: '100px', textAlign: 'right' }}
        >
          {kind}
        </Text>
        {variants.map(variant => (
          <Flex
            key={variant}
            css={{ width: '160px', justifyContent: 'center' }}
          >
            <Status kind={kind} variant={variant}>
              {kind.charAt(0).toUpperCase() + kind.slice(1)}
            </Status>
          </Flex>
        ))}
      </Flex>
    ))}
  </Flex>
);

export const WithoutIcons = () => (
  <Flex flexDirection="column" gap={6} p={4} bg="levels.surface">
    <Flex gap={6}>
      <Text
        typography="body2"
        bold
        css={{ width: '100px', textAlign: 'right' }}
      >
        {/* empty for alignment */}
      </Text>
      {variants.map(variant => (
        <Text
          key={variant}
          typography="body2"
          bold
          css={{ width: '160px', textAlign: 'center' }}
        >
          {variant}
        </Text>
      ))}
    </Flex>

    {kinds.map(kind => (
      <Flex key={kind} gap={6} alignItems="center">
        <Text
          typography="body2"
          color="text.slightlyMuted"
          css={{ width: '100px', textAlign: 'right' }}
        >
          {kind}
        </Text>
        {variants.map(variant => (
          <Flex
            key={variant}
            css={{ width: '160px', justifyContent: 'center' }}
          >
            <Status kind={kind} variant={variant} noIcon>
              {kind.charAt(0).toUpperCase() + kind.slice(1)}
            </Status>
          </Flex>
        ))}
      </Flex>
    ))}
  </Flex>
);
