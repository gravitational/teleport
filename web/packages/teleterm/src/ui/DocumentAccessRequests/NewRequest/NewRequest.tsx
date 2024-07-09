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

import { Alert, Box, Flex, Link, Text, Indicator } from 'design';
import { space, SpaceProps, width } from 'design/system';
import { Info as InfoIcon } from 'design/Icon';

import {
  ShowResources,
  Cluster,
} from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import { SearchPagination, SearchPanel } from 'shared/components/Search';
import {
  ResourceList,
  ResourceMap,
} from 'shared/components/AccessRequests/NewRequest';

import {
  PendingAccessRequest,
  extractResourceRequestProperties,
} from 'teleterm/ui/services/workspacesService/accessRequestsService';

import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import useNewRequest, { ResourceKind } from './useNewRequest';

const agentOptions: ResourceOption[] = [
  { value: 'role', label: 'Roles' },
  {
    value: 'node',
    label: 'Servers',
  },
  {
    value: 'app',
    label: 'Apps',
  },
  {
    value: 'db',
    label: 'Databases',
  },
  {
    value: 'kube_cluster',
    label: 'Kubes',
  },
];

export function NewRequest() {
  const ctx = useAppContext();

  const { rootClusterUri } = useWorkspaceContext();
  const rootCluster = ctx.clustersService.findCluster(rootClusterUri);
  if (rootCluster.showResources === ShowResources.UNSPECIFIED) {
    return <Indicator />;
  }
  return <Inner rootCluster={rootCluster} />;
}

function Inner(props: { rootCluster: Cluster }) {
  const {
    attempt,
    agentFilter,
    pageCount,
    updateQuery,
    updateSearch,
    selectedResource,
    customSort,
    fetchStatus,
    onAgentLabelClick,
    addedResources,
    addOrRemoveResource,
    updateResourceKind,
    prevPage,
    requestableRoles,
    nextPage,
    agents,
    addedItemsCount,
  } = useNewRequest(props.rootCluster);
  const { documentsService, localClusterUri } = useWorkspaceContext();

  const requestStarted = addedItemsCount > 0;

  function openClusterDocument() {
    const doc = documentsService.createClusterDocument({
      clusterUri: localClusterUri,
    });
    documentsService.add(doc);
    documentsService.open(doc.uri);
  }

  const isRequestingResourcesFromResourcesViewEnabled =
    props.rootCluster.showResources === ShowResources.REQUESTABLE;
  // This means that we can only request roles.
  // Let's hide all tabs in that case.
  const filteredAgentOptions = isRequestingResourcesFromResourcesViewEnabled
    ? []
    : agentOptions;

  const isRoleList = selectedResource === 'role';

  return (
    <Layout mx="auto" px={5} pt={3} height="100%">
      {attempt.status === 'failed' && (
        <Alert kind="danger" children={attempt.statusText} />
      )}
      <StyledMain>
        <Flex mt={3} mb={3}>
          {filteredAgentOptions.map(agent => (
            <StyledNavButton
              key={agent.value}
              mr={6}
              p={1}
              active={selectedResource === agent.value}
              onClick={() => updateResourceKind(agent.value)}
            >
              {agent.label}
            </StyledNavButton>
          ))}
        </Flex>
        {/* roles use client-side search */}
        {!isRoleList && (
          <SearchPanel
            updateQuery={updateQuery}
            updateSearch={updateSearch}
            pageIndicators={pageCount}
            filter={agentFilter}
            showSearchBar={true}
            disableSearch={fetchStatus === 'loading'}
          />
        )}
        <ResourceList
          agents={agents}
          selectedResource={selectedResource}
          requestStarted={requestStarted}
          customSort={customSort}
          onLabelClick={onAgentLabelClick}
          addedResources={toResourceMap(addedResources)}
          addOrRemoveResource={addOrRemoveResource}
          requestableRoles={requestableRoles}
          disableRows={fetchStatus === 'loading'}
        />
        {!isRoleList && (
          <SearchPagination
            nextPage={fetchStatus === 'loading' ? null : nextPage}
            prevPage={fetchStatus === 'loading' ? null : prevPage}
          />
        )}
      </StyledMain>
      {isRequestingResourcesFromResourcesViewEnabled && (
        <Alert kind="outline-info" mb={2}>
          <InfoIcon color="info" pr={2} />
          <Text>
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
          </Text>
        </Alert>
      )}
    </Layout>
  );
}

const Layout = styled(Box)`
  flex-direction: column;
  display: flex;
  flex: 1;
  max-width: 1248px;

  ::after {
    content: ' ';
    padding-bottom: 24px;
  }
`;

interface StyledNavButtonProps extends SpaceProps {
  active?: boolean;
}

const StyledNavButton = styled.button<StyledNavButtonProps>(props => {
  return {
    color: props.active
      ? props.theme.colors.text.main
      : props.theme.colors.text.slightlyMuted,
    cursor: 'pointer',
    display: 'inline-flex',
    fontSize: '14px',
    position: 'relative',
    padding: '0',
    marginRight: '24px',
    textDecoration: 'none',
    fontWeight: props.active ? 700 : 400,
    outline: 'inherit',
    border: 'none',
    backgroundColor: 'inherit',
    flexShrink: '0',
    borderRadius: '4px',
    fontFamily: 'inherit',

    '&:hover, &:focus': {
      background: props.theme.colors.spotBackground[0],
    },
    ...space(props),
    ...width(props),
  };
});

const StyledMain = styled.div`
  display: flex;
  flex-direction: column;
  flex: 1;
`;

type ResourceOption = {
  value: ResourceKind;
  label: string;
};

function toResourceMap(request: PendingAccessRequest): ResourceMap {
  const resourceMap: ResourceMap = {
    user_group: {},
    windows_desktop: {},
    role: {},
    kube_cluster: {},
    node: {},
    db: {},
    app: {},
  };
  if (request.kind === 'role') {
    request.roles.forEach(role => {
      resourceMap.role[role] = role;
    });
  }

  if (request.kind === 'resource') {
    request.resources.forEach(resourceRequest => {
      const { kind, id, name } =
        extractResourceRequestProperties(resourceRequest);
      resourceMap[kind][id] = name;
    });
  }
  return resourceMap;
}
