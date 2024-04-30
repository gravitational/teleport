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

import React, { useMemo } from 'react';

import styled from 'styled-components';

import { Alert, Box, Flex } from 'design';
import { space, width } from 'design/system';

import { SearchPagination, SearchPanel } from 'shared/components/Search';
import { ResourceList } from 'shared/components/AccessRequests/NewRequest';

import useNewRequest, { ResourceKind } from './useNewRequest';
import ChangeResourceDialog from './ChangeResourceDialog';

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
  const {
    attempt,
    agentFilter,
    pageCount,
    updateQuery,
    updateSearch,
    selectedResource,
    customSort,
    handleConfirmChangeResource,
    toResource,
    setToResource,
    fetchStatus,
    onAgentLabelClick,
    addedResources,
    addOrRemoveResource,
    updateResourceKind,
    prevPage,
    requestableRoles,
    isLeafCluster,
    nextPage,
    agents,
  } = useNewRequest();

  function handleUpdateSelectedResource(kind: ResourceKind) {
    const numAddedAgents =
      Object.keys(addedResources.node).length +
      Object.keys(addedResources.db).length +
      Object.keys(addedResources.app).length +
      Object.keys(addedResources.kube_cluster).length +
      Object.keys(addedResources.windows_desktop).length;

    const numAddedRoles = Object.keys(addedResources.role).length;

    if (
      (kind === 'role' && numAddedAgents > 0) ||
      (kind !== 'role' && numAddedRoles > 0)
    ) {
      setToResource(kind);
    } else {
      updateResourceKind(kind);
    }
  }

  // Leaf clusters do not allow role requests, so we do not show that option in the UI if leaf
  const filteredAgentOptions = useMemo(
    () =>
      agentOptions.filter(agent =>
        isLeafCluster ? agent.value !== 'role' : agent
      ),
    [isLeafCluster]
  );

  const isRoleList = selectedResource === 'role';

  return (
    <Layout mx="auto" px={5} pt={3} height="100%" flexDirection="column">
      {attempt.status === 'failed' && (
        <Alert kind="danger" children={attempt.statusText} />
      )}
      <ChangeResourceDialog
        toResource={toResource}
        onClose={() => setToResource(null)}
        onConfirm={handleConfirmChangeResource}
      />
      <StyledMain>
        <Flex mt={3} mb={3}>
          {filteredAgentOptions.map(agent => (
            <StyledNavButton
              key={agent.value}
              mr={6}
              p={1}
              active={selectedResource === agent.value}
              onClick={() => handleUpdateSelectedResource(agent.value)}
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
          customSort={customSort}
          onLabelClick={onAgentLabelClick}
          addedResources={addedResources}
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

const StyledNavButton = styled.button(props => {
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
