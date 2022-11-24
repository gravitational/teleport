import React from 'react';

import styled from 'styled-components';

import { Box, Flex, Alert } from 'design';
import { space, width } from 'design/system';

import { SearchPanel, SearchPagination } from 'shared/components/Search';
import { ResourceList } from 'e-teleport/Workflow/NewRequest/ResourceList';

import useNewRequest, { ResourceKind } from './useNewRequest';
import ChangeResourceDialog from './ChangeResourceDialog';

const agentOptions: ResourceOption[] = [
  { value: 'role', label: 'Roles' },
  {
    value: 'node',
    label: 'Servers',
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
        <StyledNav mt={3} mb={3}>
          {agentOptions.map(agent => (
            <StyledNavButton
              key={agent.value}
              mr={6}
              active={selectedResource === agent.value}
              onClick={() => handleUpdateSelectedResource(agent.value)}
            >
              {agent.label}
            </StyledNavButton>
          ))}
        </StyledNav>
        <SearchPanel
          updateQuery={updateQuery}
          updateSearch={updateSearch}
          pageCount={pageCount}
          filter={agentFilter}
          showSearchBar={true}
          disableSearch={fetchStatus === 'loading'}
        />
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
        <SearchPagination
          nextPage={fetchStatus === 'loading' ? null : nextPage}
          prevPage={fetchStatus === 'loading' ? null : prevPage}
        />
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
      ? props.theme.colors.light
      : props.theme.colors.text.secondary,
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

    '&:hover, &:focus': {
      background: props.theme.colors.primary.main,
    },
    ...space(props),
    ...width(props),
  };
});

const StyledNav = styled(Flex)`
  min-width: 180px;
  min-height: 16px;
  overflow: auto;
`;

const StyledMain = styled.div`
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  flex: 1;
`;

type ResourceOption = {
  value: ResourceKind;
  label: string;
};
