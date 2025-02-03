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

import { Fragment } from 'react';

import { Flex, Text } from 'design';

import { AccessRequestCheckoutButton } from './AccessRequestCheckoutButton';
import { ShareFeedback } from './ShareFeedback';
import { useActiveDocumentClusterBreadcrumbs } from './useActiveDocumentClusterBreadcrumbs';

export function StatusBar() {
  const breadcrumbs = useActiveDocumentClusterBreadcrumbs();

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
      gap={2}
      color="text.slightlyMuted"
      overflow="hidden"
    >
      <Flex
        css={`
          // If the breadcrumbs are wider than the available space,
          // allow scrolling them horizontally, but do not show the scrollbar.
          width: 100%;
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
                  <Text color="text.disabled">→</Text>
                )}
              </Fragment>
            ))}
          </Flex>
        )}
      </Flex>

      <Flex gap={2} alignItems="center">
        <AccessRequestCheckoutButton />
        <ShareFeedback />
      </Flex>
    </Flex>
  );
}
