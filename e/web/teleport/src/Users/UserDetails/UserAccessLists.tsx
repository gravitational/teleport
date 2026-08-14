/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { useState } from 'react';
import { Link } from 'react-router';
import styled from 'styled-components';

import { Flex, Label, Text } from 'design';
import * as Icons from 'design/Icon';
import { ShimmerBox } from 'design/ShimmerBox';
import { HoverTooltip } from 'design/Tooltip';
import { AccessListUserAssignmentType } from 'gen-proto-ts/teleport/accesslist/v1/accesslist_pb';
import { ErrorSuspenseWrapper } from 'shared/components/ErrorSuspenseWrapper/ErrorSuspenseWrapper';
import { useInfiniteScroll } from 'shared/hooks/useInfiniteScroll';

import cfg from 'e-teleport/config';
import {
  AccessListUserAssignments,
  useSuspenseInfiniteUserAccessLists,
} from 'e-teleport/services/accessmanagement';
import useTeleport from 'e-teleport/useTeleportE';
import {
  ClickableLabel,
  ExpandableContainer,
  SectionParagraph,
  SectionTitle,
  UserDetailsSectionProps,
} from 'teleport/Users/UserDetails/UserDetails';

function AccessListsLoading() {
  return (
    <>
      <SectionTitle>Access Lists</SectionTitle>
      <SectionParagraph>
        <Flex flexWrap="wrap" gap={2}>
          <ShimmerBox width="100px" height="20px" />
          <ShimmerBox width="120px" height="20px" />
          <ShimmerBox width="90px" height="20px" />
        </Flex>
      </SectionParagraph>
    </>
  );
}

function AccessLists({ user }: UserDetailsSectionProps) {
  const ctx = useTeleport();
  const perms = ctx.storeUser.getAccessListAccess();

  const [isExpanded, setIsExpanded] = useState(false);
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, error } =
    useSuspenseInfiniteUserAccessLists(
      { username: user?.name || '', pageSize: 100 },
      {
        initialPageParam: '',
        getNextPageParam: data => data?.nextKey || undefined,
      }
    );

  const { setTrigger } = useInfiniteScroll({
    fetch: async () => {
      if (isExpanded && hasNextPage && !isFetchingNextPage && !error) {
        fetchNextPage();
      }
    },
  });

  const canList = perms.list && perms.read;

  if (!canList) {
    return null;
  }

  const initialItemCount = 7;
  const accessLists = data?.pages.flatMap(page => page.accessLists) || [];
  const totalCount = data?.pages[0]?.totalCount || 0;
  const accessListsToShow = isExpanded
    ? accessLists
    : accessLists.slice(0, initialItemCount);
  const hasMoreAccessLists = totalCount > initialItemCount;

  return (
    <>
      <SectionTitle>Access Lists ({totalCount.toLocaleString()})</SectionTitle>
      <SectionParagraph>
        {accessLists && accessLists.length > 0 && (
          <>
            <ExpandableContainer isExpanded={isExpanded}>
              <Flex flexWrap="wrap" rowGap={1} columnGap={2}>
                {accessListsToShow.map(accessList => {
                  const tooltipText = renderAssignmentText(
                    accessList.userAssignments
                  );

                  const labelElement = (
                    <LinkableLabel
                      kind="secondary"
                      to={cfg.getAccessListManagementRoute(accessList.id)}
                    >
                      <Flex alignItems="center" gap={1}>
                        <Icons.ListAddCheck size={16} />
                        {accessList.title}
                      </Flex>
                    </LinkableLabel>
                  );

                  return tooltipText ? (
                    <HoverTooltip key={accessList.id} tipContent={tooltipText}>
                      {labelElement}
                    </HoverTooltip>
                  ) : (
                    <div key={accessList.id}>{labelElement}</div>
                  );
                })}
                {hasMoreAccessLists && !isExpanded && (
                  <ClickableLabel
                    kind="secondary"
                    onClick={() => setIsExpanded(!isExpanded)}
                  >
                    + {(totalCount - initialItemCount).toLocaleString()} more
                  </ClickableLabel>
                )}
              </Flex>
              {isExpanded && (
                <>
                  {isFetchingNextPage && (
                    <Flex mt={2} gap={2}>
                      <ShimmerBox width="100px" height="20px" />
                      <ShimmerBox width="120px" height="20px" />
                    </Flex>
                  )}
                  <div ref={setTrigger} />
                </>
              )}
            </ExpandableContainer>
            {hasMoreAccessLists && isExpanded && (
              <Flex mt={2} alignItems="center" justifyContent="space-between">
                <ClickableLabel
                  kind="secondary"
                  onClick={() => setIsExpanded(!isExpanded)}
                >
                  Show less
                </ClickableLabel>
                <Text fontSize={12} color="text.muted">
                  Showing {accessLists.length.toLocaleString()} of{' '}
                  {totalCount.toLocaleString()}
                </Text>
              </Flex>
            )}
          </>
        )}
        {accessLists && accessLists.length === 0 && (
          <Text color="text.muted">No access lists assigned.</Text>
        )}
      </SectionParagraph>
    </>
  );
}

export function UserAccessLists({ user }: UserDetailsSectionProps) {
  return (
    <ErrorSuspenseWrapper
      errorComponent={() => null}
      loadingComponent={AccessListsLoading}
    >
      <AccessLists user={user} />
    </ErrorSuspenseWrapper>
  );
}

function renderAssignmentText(
  userAssignments?: AccessListUserAssignments
): string {
  if (!userAssignments) return '';

  const { ownershipType, membershipType } = userAssignments;
  const type = ownershipType || membershipType;
  const role = ownershipType ? 'Owner' : 'Member';
  const suffix =
    type === AccessListUserAssignmentType.INHERITED ? ' (Inherited)' : '';

  return type ? `${role}${suffix}` : '';
}

const LinkableLabel = styled(Label).attrs<{ to: string }>(({ to }) => ({
  as: Link,
  to,
}))<{ to: string }>`
  cursor: pointer;
  text-decoration: none;

  &:hover {
    background: ${props => props.theme.colors.levels.elevated};
  }
`;
