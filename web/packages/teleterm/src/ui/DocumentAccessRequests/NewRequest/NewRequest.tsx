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

import styled from 'styled-components';
import { Alert, Box, Link } from 'design';

import { ShowResources } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import {
  ResourceList,
  ResourceMap,
} from 'shared/components/AccessRequests/NewRequest';

import { PendingAccessRequest } from 'teleterm/ui/services/workspacesService/accessRequestsService';

import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { useAppContext } from 'teleterm/ui/appContextProvider';

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
  const requestStarted = accessRequestsService.getAddedItemsCount() > 0;

  function openClusterDocument() {
    const doc = documentsService.createClusterDocument({
      clusterUri: localClusterUri,
    });
    documentsService.add(doc);
    documentsService.open(doc.uri);
  }

  const doesUnifiedResourcesShowBothAccesibleAndRequestableResources =
    rootCluster?.showResources === ShowResources.REQUESTABLE;

  return (
    <Layout mx="auto" px={5} pt={3} height="100%">
      <StyledMain>
        <ResourceList
          agents={[]}
          selectedResource={'role'}
          requestStarted={requestStarted}
          customSort={undefined}
          onLabelClick={() => {}}
          addedResources={toResourceMap(addedResources)}
          addOrRemoveResource={(kind, resourceId) => {
            // We can only have roles here.
            if (kind === 'role') {
              accessRequestsService.addOrRemoveRole(resourceId);
            }
          }}
          requestableRoles={requestableRoles}
          disableRows={false}
        />
      </StyledMain>
      <Alert kind="outline-info" mb={2}>
        {doesUnifiedResourcesShowBothAccesibleAndRequestableResources ? (
          <>
            To request access to a resource, go to the{' '}
            {/*TODO: Improve ButtonLink to look more like a text, then use it instead of the Link. */}
            <Link
              css={`
                cursor: pointer;
                color: inherit !important;
              `}
              onClick={openClusterDocument}
            >
              resources view
            </Link>{' '}
            or find it in the search bar.
          </>
        ) : (
          <>
            To request access to a resource, go to the{' '}
            {/*TODO: Improve ButtonLink to look more like a text, then use it instead of the Link. */}
            <Link
              css={`
                cursor: pointer;
                color: inherit !important;
              `}
              onClick={openClusterDocument}
            >
              resources view
            </Link>{' '}
            and select Access Requests &gt; Show requestable resources.
          </>
        )}
      </Alert>
    </Layout>
  );
}

const Layout = styled(Box)`
  flex-direction: column;
  display: flex;
  flex: 1;
  max-width: 1248px;

  &::after {
    content: ' ';
    padding-bottom: 24px;
  }
`;

const StyledMain = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1;
`;

function toResourceMap(request: PendingAccessRequest): ResourceMap {
  const resourceMap: ResourceMap = {
    user_group: {},
    windows_desktop: {},
    role: {},
    kube_cluster: {},
    node: {},
    db: {},
    app: {},
    saml_idp_service_provider: {},
    namespace: {},
  };
  if (request.kind === 'role') {
    request.roles.forEach(role => {
      resourceMap.role[role] = role;
    });
  }

  return resourceMap;
}
