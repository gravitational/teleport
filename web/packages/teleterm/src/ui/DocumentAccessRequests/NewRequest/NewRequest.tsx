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

import { Box, Flex, Link } from 'design';
import { Info } from 'design/Alert';
import { ShowResources } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { Roles } from 'shared/components/AccessRequests/NewRequest';
import { useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';

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
