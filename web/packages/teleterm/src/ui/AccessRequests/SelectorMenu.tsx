/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { formatDistanceToNow, isPast } from 'date-fns';
import { ComponentType, useEffect, useMemo, useRef, useState } from 'react';
import styled, { css, useTheme } from 'styled-components';

import { Box, ButtonText, Flex, Indicator, P3, Popover } from 'design';
import * as Icon from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import { MenuItemSectionLabel } from 'design/Menu/MenuItem';
import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import { AccessRequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/access_request_pb';
import { RequestableResourceKind } from 'shared/components/AccessRequests/NewRequest';
import { getResourcesOrRolesFromRequest } from 'shared/components/AccessRequests/Shared/Shared';
import {
  Attempt,
  mapAttempt,
  useAsync,
  useDelayedRepeatedAttempt,
} from 'shared/hooks/useAsync';
import { AccessRequest as SharedAccessRequest } from 'shared/services/accessRequests';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  Menu,
  MenuItemContainer,
  MenuListItem,
  Separator,
} from 'teleterm/ui/components/Menu';
import { useResourcesContext } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { useLoggedInUser } from 'teleterm/ui/hooks/useLoggedInUser';
import { TopBarButton } from 'teleterm/ui/TopBar/TopBarButton';
import { retryWithRelogin } from 'teleterm/ui/utils';

import {
  ConnectionStatusIndicator,
  Status,
} from '../TopBar/Connections/ConnectionsFilterableList/ConnectionStatusIndicator';
import { useAccessRequestsContext } from './AccessRequestsContext';

export function SelectorMenu() {
  const [open, setOpen] = useState(false);
  // Captures a snapshot of the assumed request when the menu opens.
  // Used exclusively for sorting, to make assumed requests stay on top.
  // Prevents requests from shifting when they are assumed or dropped
  // while the menu is open.
  const [assumedSnapshot, setAssumedSnapshot] = useState<
    Map<string, AccessRequest>
  >(() => new Map());
  const {
    canUse,
    assumed,
    fetchRequestsAttempt: eagerFetchRequestsAttempt,
    fetchRequests,
  } = useAccessRequestsContext();
  const fetchRequestsAttempt = useDelayedRepeatedAttempt(
    eagerFetchRequestsAttempt
  );
  const { rootClusterUri } = useWorkspaceContext();
  const ctx = useAppContext();
  const { clustersService } = ctx;
  const selectorRef = useRef<HTMLButtonElement>();
  const { requestResourcesRefresh } = useResourcesContext(rootClusterUri);
  const loggedInUser = useLoggedInUser();
  const username = loggedInUser?.name;

  // Returns only our own requests that are in the approved state.
  const assumableRequests = useMemo(
    () =>
      mapAttempt(fetchRequestsAttempt, requests => {
        return requests.filter(
          d => d.state === 'APPROVED' && d.user === username
        );
      }),
    [fetchRequestsAttempt, username]
  );

  // Ensure that the assumed requests are always displayed in the menu.
  // It's possible that the user assumed a request that was later deleted.
  // If they refreshed the list after that, the request would be gone there, but
  // still displayed in the status bar.
  // The same thing would happen if the API call to fetch requests failed.
  // We need to make sure that the assumed roles are always visible.
  const assumedAndAssumableRequests = useMemo(() => {
    const allRequests = [...(assumableRequests.data || [])];
    assumed.forEach(assumedRequest => {
      if (
        !allRequests.find(
          fetchedRequest => fetchedRequest.id === assumedRequest.id
        )
      ) {
        allRequests.push(assumedRequest);
      }
    });
    return allRequests;
  }, [assumableRequests.data, assumed]);

  // Keeps the assumed requests on top of the list.
  // It's important to sort using `assumedSnapshot` object, see a comment there.
  const sortedRequests = useMemo(
    () =>
      assumedAndAssumableRequests.toSorted((a, b) =>
        assumedSnapshot.get(a.id) === assumedSnapshot.get(b.id)
          ? 0
          : assumedSnapshot.get(a.id)
            ? -1
            : 1
      ),
    [assumedAndAssumableRequests, assumedSnapshot]
  );

  useEffect(() => {
    if (canUse) {
      void fetchRequests();
    }
  }, [canUse, fetchRequests]);

  if (!canUse) {
    return;
  }

  const documentsService =
    ctx.workspacesService.getWorkspaceDocumentService(rootClusterUri);
  const menuItems = [
    {
      title: 'View Access Requests',
      Icon: Icon.ListAddCheck,
      onNavigate: () => {
        const doc = documentsService.createAccessRequestDocument({
          clusterUri: rootClusterUri,
          state: 'browsing',
        });
        documentsService.add(doc);
        documentsService.open(doc.uri);
      },
    },
    {
      title: 'New Role Request',
      Icon: Icon.Add,
      onNavigate: () => {
        const doc = documentsService.createAccessRequestDocument({
          clusterUri: rootClusterUri,
          state: 'creating',
        });
        documentsService.add(doc);
        documentsService.open(doc.uri);
      },
    },
  ].map(item => {
    return (
      <MenuListItem
        key={item.title}
        item={item}
        closeMenu={() => setOpen(false)}
      />
    );
  });

  function openMenu(): void {
    void fetchRequests();
    setAssumedSnapshot(assumed);
    setOpen(true);
  }

  function closeMenu(): void {
    setOpen(false);
  }

  function viewRequest(requestId: string): void {
    const doc = documentsService.createAccessRequestDocument({
      clusterUri: rootClusterUri,
      requestId,
      state: 'reviewing',
    });
    documentsService.add(doc);
    documentsService.open(doc.uri);
  }

  async function assumeOrDrop(requestId: string): Promise<void> {
    await retryWithRelogin(ctx, rootClusterUri, async () => {
      if (assumed.has(requestId)) {
        await clustersService.dropRoles(rootClusterUri, [requestId]);
      } else {
        await clustersService.assumeRoles(rootClusterUri, [requestId]);
      }
    });
    requestResourcesRefresh();
  }

  const isResourceRequestAssumed = sortedRequests
    .filter(a => assumed.has(a.id))
    .some(a => a.resources.length);

  const fetchRequestsStatusText = getFetchRequestsStatusText(
    fetchRequestsAttempt,
    sortedRequests
  );

  return (
    <>
      <TopBarButton
        ref={selectorRef}
        isOpened={open}
        title="Access Requests"
        id="access-requests-menu"
        onClick={open ? closeMenu : openMenu}
      >
        <Icon.ListAddCheck size="medium" />
      </TopBarButton>
      <Popover
        open={open}
        anchorEl={selectorRef.current}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        onClose={closeMenu}
        popoverCss={() => `max-width: min(560px, 90%)`}
      >
        <Menu
          css={`
            min-width: 400px;
          `}
        >
          {menuItems}
          <Separator />
          <MenuItemSectionLabel
            css={`
              justify-content: space-between;
            `}
            color="text.muted"
            mx={2}
            my={1}
          >
            Available Requests
            {fetchRequestsAttempt.status === 'processing' ? (
              <Indicator
                delay="none"
                size="small"
                // Aligns the indicator to the button.
                mr={2}
                css={`
                  display: flex;
                `}
              />
            ) : (
              <ButtonText
                onClick={() => {
                  //TODO(gzdunek): Allow useDelayedRepeatedAttempt to show feedback immediately.
                  // Explicit calls to refresh the list shouldn't depend on the delayed
                  // behavior, and instead show feedback immediately.
                  void fetchRequests();
                }}
                size="small"
                title="Refresh"
              >
                <Icon.Refresh size="small" />
              </ButtonText>
            )}
          </MenuItemSectionLabel>
          {fetchRequestsStatusText && (
            <MenuListItem
              item={{
                title: fetchRequestsStatusText,
                disabledText: fetchRequestsStatusText,
                isDisabled: true,
              }}
              closeMenu={closeMenu}
            />
          )}
          {sortedRequests.map(a => {
            const isAssumed = assumed.has(a.id);
            return (
              <RequestItem
                key={a.id}
                request={a}
                isAssumed={isAssumed}
                isResourceRequestAssumed={isResourceRequestAssumed}
                fetchRequestsAttempt={fetchRequestsAttempt}
                assumeOrDrop={() => assumeOrDrop(a.id)}
                view={() => {
                  viewRequest(a.id);
                  closeMenu();
                }}
              />
            );
          })}
        </Menu>
      </Popover>
    </>
  );
}

function RequestItem(props: {
  isAssumed: boolean;
  fetchRequestsAttempt: Attempt<AccessRequest[]>;
  request: AccessRequest;
  isResourceRequestAssumed: boolean;
  assumeOrDrop(): Promise<unknown>;
  view(): void;
}) {
  const [assumeOrDropAttempt, runAssumeOrDrop] = useAsync(props.assumeOrDrop);
  const isResourceRequest = !!props.request.resources.length;
  // We can assume only one resource request.
  const canAssumeResourceRequest =
    !props.isResourceRequestAssumed || !isResourceRequest || props.isAssumed;

  const items = getResourcesOrRolesFromRequest(
    makeSharedRequest(props.request)
  );
  let title = items.map(i => i.title).join(', ');
  if (props.request.resources.length) {
    // Show the role name too.
    title += ` (${props.request.roles.join(', ')})`;
  }
  if (canAssumeResourceRequest) {
    title = props.isAssumed
      ? `Drop the request for ${title}`
      : `Assume the request for ${title}`;
  }
  const theme = useTheme();
  const isDisabled =
    props.fetchRequestsAttempt.status === 'processing' ||
    assumeOrDropAttempt.status === 'processing' ||
    !canAssumeResourceRequest;

  return (
    <StyledMenuItemContainer
      assumed={props.isAssumed}
      disabled={isDisabled}
      onClick={() => !isDisabled && void runAssumeOrDrop()}
      title={title}
    >
      <Flex alignItems="center">
        <ConnectionStatusIndicator
          // Aligns margins of the indicator to match spacing of the regular item with an icon.
          ml={2}
          mr="20px"
          status={getAccessRequestIconStatus(
            assumeOrDropAttempt,
            props.isAssumed
          )}
          activeStatusColor={theme.colors.interactive.solid.primary.default}
        />
        <Box
          css={`
            line-height: 1.3;
          `}
        >
          <Flex gap={1} flexWrap="wrap">
            {clipRequestItems(items).map((i, index, array) => {
              const { Icon, name } = i;
              const isLast = index === array.length - 1;
              return (
                <Flex key={`name-${index}`} gap={1}>
                  {Icon && <Icon size="small" />}
                  {name}
                  {!isLast && ','}
                </Flex>
              );
            })}
          </Flex>
          <P3
            css={`
              white-space: normal;
            `}
          >
            {getAccessRequestStatusText({
              canAssumeResourceRequest,
              attempt: assumeOrDropAttempt,
              expires: props.request.expires,
              isAssumed: props.isAssumed,
            })}
          </P3>
        </Box>
      </Flex>
      <ButtonText
        size="small"
        css={`
          visibility: hidden;
          transition: none;
        `}
        ml={2}
        title="View Request"
        className="info"
        onClick={e => {
          props.view();
          e.stopPropagation();
        }}
      >
        <Icon.Info size="small" />
      </ButtonText>
    </StyledMenuItemContainer>
  );
}

function getFetchRequestsStatusText(
  attempt: Attempt<unknown>,
  assumable: AccessRequest[]
) {
  switch (attempt.status) {
    case '':
    case 'success':
      if (!assumable.length) {
        return 'No requests to assume.';
      }
      return;
    case 'processing':
      if (!assumable.length) {
        return 'Loading…';
      }
      return;
    case 'error':
      return `Could not fetch available requests: ${attempt.statusText}`;
  }
}

function getAccessRequestIconStatus(
  attempt: Attempt<unknown>,
  isAssumed: boolean
): Status {
  switch (attempt.status) {
    case 'error':
      return 'error';
    case 'processing':
      return 'processing';
    case '':
    case 'success': {
      return isAssumed ? 'on' : 'off';
    }
  }
}

function getAccessRequestStatusText(args: {
  attempt: Attempt<unknown>;
  expires: Timestamp;
  isAssumed: boolean;
  canAssumeResourceRequest: boolean;
}) {
  const expirationDate = Timestamp.toDate(args.expires);
  const expiresIn = isPast(expirationDate)
    ? 'Expired'
    : `Expires in ${formatDistanceToNow(Timestamp.toDate(args.expires))}`;

  if (args.attempt.status === 'error') {
    return `Could not update access: ${args.attempt.statusText}`;
  }

  if (args.isAssumed) {
    return `Access assumed · ${expiresIn}`;
  }

  if (!args.canAssumeResourceRequest) {
    return 'Another Resource Access Request is assumed.';
  }

  return expiresIn;
}

const StyledMenuItemContainer = styled(MenuItemContainer)<{ assumed: boolean }>`
  ${props =>
    props.assumed &&
    css`
      background: ${props.theme.colors.interactive.tonal.primary.at(1)};
      &:hover {
        background-color: ${props.theme.colors.interactive.tonal.primary.at(0)};
      }
    `};

  &:hover .info {
    visibility: visible;
  }

  padding-block: ${props => props.theme.space[2]}px;
  justify-content: space-between;
`;

/** Casts request kind string to a union. */
function makeSharedRequest(
  request: AccessRequest
): Pick<SharedAccessRequest, 'resources' | 'roles'> {
  return {
    ...request,
    resources: request.resources.map(r => ({
      ...r,
      id: {
        ...r.id,
        kind: r.id.kind as RequestableResourceKind,
      },
    })),
  };
}

const MAX_ITEMS_TO_SHOW_IN_REQUEST = 5;

interface RequestItem {
  Icon: ComponentType<IconProps> | undefined;
  name: string;
}

/**
 * Returns up to `MAX_ITEMS_TO_SHOW_IN_REQUEST` roles or resources.
 * If the total exceeds this limit, an additional "+n more" is added.
 */
function clipRequestItems(items: RequestItem[]): RequestItem[] {
  // We should rather detect how much space we have,
  // but for simplicity we only count items.
  const moreToShow = Math.max(items.length - MAX_ITEMS_TO_SHOW_IN_REQUEST, 0);
  const clippedItems = items.slice(0, MAX_ITEMS_TO_SHOW_IN_REQUEST);
  if (moreToShow) {
    clippedItems.push({
      Icon: undefined,
      name: `+${moreToShow} more`,
    });
  }

  return clippedItems;
}
