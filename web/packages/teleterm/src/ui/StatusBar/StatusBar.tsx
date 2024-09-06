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

import React from 'react';
import { Flex, Text } from 'design';

import { useActiveDocumentClusterBreadcrumbs } from './useActiveDocumentClusterBreadcrumbs';
import { ShareFeedback } from './ShareFeedback';
import { AccessRequestCheckoutButton } from './AccessRequestCheckoutButton';

export function StatusBar() {
  const clusterBreadcrumbs = useActiveDocumentClusterBreadcrumbs();

  return (
    <Flex
      width="100%"
      height="28px"
      css={`
        border-top: 1px solid ${props => props.theme.colors.spotBackground[1]};
      `}
      alignItems="center"
      justifyContent="space-between"
      px={2}
      overflow="hidden"
    >
      <Text
        color="text.slightlyMuted"
        fontSize="14px"
        css={`
          white-space: nowrap;
        `}
        title={clusterBreadcrumbs}
      >
        {clusterBreadcrumbs}
      </Text>
      <Flex gap={2} alignItems="center">
        <AccessRequestCheckoutButton />
        <ShareFeedback />
      </Flex>
    </Flex>
  );
}
