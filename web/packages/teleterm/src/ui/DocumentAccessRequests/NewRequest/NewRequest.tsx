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

import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';

import { Box, Flex, Link } from 'design';
import { Info } from 'design/Alert';
import Table from 'design/DataTable';
import { Page } from 'design/DataTable/types';
import { ShowResources } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { Roles } from 'shared/components/AccessRequests/NewRequest';
import { useAsync } from 'shared/hooks/useAsync';

import { cloneAbortSignal } from 'teleterm/services/tshd';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';

const pageSize = 20;

/**
 * Only allows requesting roles (resources can be requested through the unified
 * resources or the search bar).
 *
 * Available via Additional actions -> New role request.
 */
export function NewRequest() {
  const ctx = useAppContext();

  const {
    rootClusterUri,
    localClusterUri,
    documentsService,
    accessRequestsService,
  } = useWorkspaceContext();
  const rootCluster = ctx.clustersService.findCluster(rootClusterUri);
  ctx.clustersService.useState();

  const loggedInUser = rootCluster?.loggedInUser;
  const requestableRoles = loggedInUser?.requestableRoles || [];
  const addedResources = accessRequestsService.getPendingAccessRequest();

  function openClusterDocument() {
    const doc = documentsService.createClusterDocument({
      clusterUri: localClusterUri,
    });
    documentsService.add(doc);
    documentsService.open(doc.uri);
  }

  const doesUnifiedResourcesShowBothAccessibleAndRequestableResources =
    rootCluster?.showResources === ShowResources.REQUESTABLE;

  const [addOrRemoveRoleAttempt, addOrRemoveRole] = useAsync((role: string) =>
    accessRequestsService.addOrRemoveRole(role)
  );

  // TODO: Move this code to a new component (RequestableRoles within this dir?).
  // TODO: Add search.
  // TODO: When it's time to fetch the next page (that is, the user clicks the button to go to the
  // next page), in the event handler we should probably use a single setPage which
  // both:
  //   1) increments index
  //   2) adds next_page_token to keys
  // This way we can still modify page.keys without firing setState from within useQuery.
  // https://tkdodo.eu/blog/breaking-react-querys-api-on-purpose#state-syncing
  const [page, setPage] = useState<Page>({ keys: [], index: 0 });
  const currentPageToken = page.keys.at(page.index);
  const { data: reqRolesResp } = useQuery({
    queryKey: [
      'requestable-roles',
      rootClusterUri,
      page.index,
      page.keys.at(page.index),
    ],
    queryFn: async ({ signal }) => {
      const { response } = await ctx.tshd.listRequestableRoles(
        {
          rootClusterUri,
          pageSize,
          pageToken: currentPageToken,
        },
        { abort: cloneAbortSignal(signal) }
      );
      return response;
    },
  });

  return (
    <Flex
      mx="auto"
      flexDirection="column"
      justifyContent="space-between"
      p={3}
      gap={3}
      width="100%"
      maxWidth="1248px"
    >
      <Box>
        <Table
          data={reqRolesResp?.roles}
          pagination={{
            pagerPosition: 'top',
            pageSize,
          }}
          isSearchable={true}
          columns={[
            // TODO: Make the name column wider.
            { key: 'name', headerText: 'Name' },
            { key: 'description', headerText: 'Description' },
          ]}
          emptyText="TODO"
        />
      </Box>
      {/* TODO: When a response comes back with NotImplemented, display the old list of roles. */}
      {/* TODO: Copy tests of the old role list to the new table. */}
      <Box
        css={`
          display: none;
        `}
      >
        <Roles
          requestable={requestableRoles}
          requested={
            addedResources.kind === 'role' ? addedResources.roles : new Set()
          }
          onToggleRole={role => void addOrRemoveRole(role)}
          disabled={addOrRemoveRoleAttempt.status === 'processing'}
        />
      </Box>
      <Info mb={0}>
        To request access to a resource, go to{' '}
        {/*TODO: Improve ButtonLink to look more like a text, then use it instead of the Link. */}
        <Link
          css={`
            cursor: pointer;
            color: inherit !important;
          `}
          onClick={openClusterDocument}
        >
          the resources view
        </Link>{' '}
        {doesUnifiedResourcesShowBothAccessibleAndRequestableResources
          ? 'or find it in the search bar.'
          : 'and select Access Requests > Show requestable resources.'}
      </Info>
    </Flex>
  );
}
