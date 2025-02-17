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

import { Fragment, useCallback } from 'react';
import { useTheme } from 'styled-components';

import { ChevronRight } from 'design/Icon';
import { ButtonPrimary, Flex, Text } from 'design';

import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import { getAssumedRequests } from 'teleterm/ui/services/clusters';
import { ConnectionStatusIndicator } from 'teleterm/ui/TopBar/Connections/ConnectionsFilterableList/ConnectionStatusIndicator';

import { AccessRequestCheckoutButton } from './AccessRequestCheckoutButton';
import { ShareFeedback } from './ShareFeedback';
import { useActiveDocumentClusterBreadcrumbs } from './useActiveDocumentClusterBreadcrumbs';

export function StatusBar(props: { onAssumedRolesClick(): void }) {
  const breadcrumbs = useActiveDocumentClusterBreadcrumbs();
  const theme = useTheme();
  const rootClusterUri = useStoreSelector(
    'workspacesService',
    useCallback(store => store.rootClusterUri, [])
  );
  const assumedRoles = Object.values(
    useStoreSelector(
      'clustersService',
      useCallback(
        state => getAssumedRequests(state, rootClusterUri),
        [rootClusterUri]
      )
    )
  );
  const assumedRolesText = assumedRoles.flatMap(r => r.roles).join(', ');

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
      gap={3}
      color="text.slightlyMuted"
      overflow="hidden"
    >
      <Flex
        css={`
          // If the breadcrumbs are wider than the available space,
          // allow scrolling them horizontally, but do not show the scrollbar.
          overflow: scroll;

          &::-webkit-scrollbar {
            display: none;
          }
        `}
      >
        {breadcrumbs && (
          <Flex
            gap={2}
            css={`
              flex-shrink: 0;
              font-size: 13px;
            `}
            title={breadcrumbs.map(({ name }) => name).join(' → ')}
          >
            {breadcrumbs.map((breadcrumb, index) => (
              <Fragment key={`${index}-${breadcrumb.name}`}>
                {breadcrumb.Icon && (
                  <breadcrumb.Icon color="text.muted" size="small" mr={-1} />
                )}
                <Text>{breadcrumb.name}</Text>
                {index !== breadcrumbs.length - 1 && (
                  // Size 'small' is too large here.
                  <ChevronRight size={13} color="text.muted" />
                )}
              </Fragment>
            ))}
          </Flex>
        )}
      </Flex>

      <Flex
        gap={1}
        alignItems="center"
        justifyContent="flex-end"
        css={`
          // Allows the content to shrink.
          min-width: 0;
        `}
      >
        {!!assumedRoles.length && (
          <ButtonPrimary
            css={`
              min-width: 40px;
            `}
            gap={2}
            title={assumedRolesText}
            size="small"
            onClick={props.onAssumedRolesClick}
          >
            <ConnectionStatusIndicator
              status="on"
              activeStatusColor={theme.colors.text.primaryInverse}
            />
            <Text
              css={`
                white-space: nowrap;
              `}
            >
              {assumedRolesText}
            </Text>
          </ButtonPrimary>
        )}
        <AccessRequestCheckoutButton />
        <ShareFeedback />
      </Flex>
    </Flex>
  );
}
