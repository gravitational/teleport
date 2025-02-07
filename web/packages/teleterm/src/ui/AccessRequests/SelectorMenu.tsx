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
import { Fragment, useEffect, useMemo, useRef, useState } from 'react';
import styled, { css, useTheme } from 'styled-components';

import { ButtonText, Flex, P3, Popover } from 'design';
import * as Icon from 'design/Icon';
import { MenuItemSectionLabel } from 'design/Menu/MenuItem';
import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import { AccessRequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/access_request_pb';
import { RequestableResourceKind } from 'shared/components/AccessRequests/NewRequest';
import { getResourcesOrRolesFromRequest } from 'shared/components/AccessRequests/Shared/Shared';
import { Attempt, mapAttempt, useAsync } from 'shared/hooks/useAsync';
import { AccessRequest as SharedAccessRequest } from 'shared/services/accessRequests';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  Menu,
  MenuItemContainer,
  MenuListItem,
  Separator,
} from 'teleterm/ui/components/Menu';
import { useResourcesContext } from 'teleterm/ui/DocumentCluster/resourcesContext';
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
    fetchRequestsAttempt,
    fetchRequests,
    rootClusterUri,
  } = useAccessRequestsContext();
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
  // The same thing will happen if the entire access request fails.
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
        data-testid="access-requests-icon"
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
              &:hover .refresh {
                visibility: visible;
              }
            `}
            color="text.muted"
            ml="8px"
            mr={2}
            my={1}
          >
            Available Requests
            <ButtonText
              size="small"
              title="Refresh"
              css={`
                visibility: hidden;
              `}
              className="refresh"
            >
              <Icon.Refresh
                size="small"
                onClick={() => {
                  void fetchRequests();
                }}
              />
            </ButtonText>
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
  const theme = useTheme();
  return (
    <StyledMenuItemContainer
      assumed={props.isAssumed}
      disabled={
        props.fetchRequestsAttempt.status === 'processing' ||
        assumeOrDropAttempt.status === 'processing' ||
        !canAssumeResourceRequest
      }
      onClick={() => void runAssumeOrDrop()}
      title={title}
    >
      <Flex gap={3} alignItems="center">
        <ConnectionStatusIndicator
          ml={2}
          mr="3px"
          status={getAccessRequestIconStatus(
            assumeOrDropAttempt,
            props.isAssumed
          )}
          activeStatusColor={theme.colors.interactive.solid.primary.default}
        />
        <Flex
          flexDirection="column"
          css={`
            line-height: 1.25;
          `}
        >
          <Flex gap={1}>
            {items.map((i, index) => {
              const { Icon, name } = i;
              const isLast = index === items.length - 1;
              return (
                <Fragment key={`name-${index}`}>
                  {Icon && <Icon size="small" />}
                  {name}
                  {!isLast && ','}
                </Fragment>
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
        </Flex>
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
        <Icon.Info size="medium" />
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
      return 'Loading…';
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
