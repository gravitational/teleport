/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
import React from 'react';
import Table, {
  Cell,
  ClickableLabelCell,
  StyledTableWrapper,
} from 'design/DataTable';
import { ButtonPrimary } from 'design';
import { Danger } from 'design/Alert';
import * as icons from 'design/Icon';
import { SearchPanel, SearchPagination } from 'shared/components/Search';

import { useWorkspaceLoggedInUser } from 'teleterm/ui/hooks/useLoggedInUser';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { useConnectMyComputerContext } from 'teleterm/ui/ConnectMyComputer';
import { Server } from 'teleterm/services/tshd/types';

import { DarkenWhileDisabled } from '../DarkenWhileDisabled';
import { getEmptyTableStatus, getEmptyTableText } from '../getEmptyTableText';
import { useClusterContext } from '../../clusterContext';

import { ConnectServerActionButton } from '../../actionButtons';

import { useServers, State } from './useServers';

export default function Container() {
  const state = useServers();
  return <ServerList {...state} />;
}

function ServerList(props: State) {
  const {
    fetchAttempt,
    agentFilter,
    pageCount,
    customSort,
    prevPage,
    nextPage,
    updateQuery,
    onAgentLabelClick,
    updateSearch,
  } = props;
  const { documentsService, rootClusterUri } = useWorkspaceContext();
  const { clusterUri } = useClusterContext();
  const loggedInUser = useWorkspaceLoggedInUser();
  const { canUse: hasPermissionsForConnectMyComputer, agentCompatibility } =
    useConnectMyComputerContext();

  const servers = fetchAttempt.data?.agentsList || [];
  const disabled = fetchAttempt.status === 'processing';
  const isRootCluster = clusterUri === rootClusterUri;
  const canAddResources = isRootCluster && loggedInUser?.acl?.tokens.create;

  const emptyTableStatus = getEmptyTableStatus(
    fetchAttempt.status,
    agentFilter.search || agentFilter.query,
    canAddResources
  );
  const canUseConnectMyComputer =
    isRootCluster &&
    hasPermissionsForConnectMyComputer &&
    agentCompatibility === 'compatible';
  let { emptyText, emptyHint } = getEmptyTableText(emptyTableStatus, 'servers');
  let emptyButton: JSX.Element;

  if (
    emptyTableStatus.status === 'no-resources' &&
    emptyTableStatus.showEnrollingResourcesHint &&
    canUseConnectMyComputer
  ) {
    emptyHint =
      'You can add them in the Teleport Web UI or by connecting your computer to the cluster.';
    emptyButton = (
      <ButtonPrimary
        type="button"
        gap={2}
        onClick={() => {
          documentsService.openConnectMyComputerDocument({ rootClusterUri });
        }}
      >
        <icons.Laptop size={'medium'} />
        Connect My Computer
      </ButtonPrimary>
    );
  }

  return (
    <>
      {fetchAttempt.status === 'error' && (
        <Danger>{fetchAttempt.statusText}</Danger>
      )}
      <StyledTableWrapper borderRadius={3}>
        <SearchPanel
          updateQuery={updateQuery}
          updateSearch={updateSearch}
          pageIndicators={pageCount}
          filter={agentFilter}
          showSearchBar={true}
          disableSearch={disabled}
        />
        <DarkenWhileDisabled disabled={disabled}>
          <Table
            columns={[
              {
                key: 'hostname',
                headerText: 'Hostname',
                isSortable: true,
              },
              {
                key: 'addr',
                headerText: 'Address',
                isSortable: false,
                render: renderAddressCell,
              },
              {
                key: 'labelsList',
                headerText: 'Labels',
                render: ({ labelsList }) => (
                  <ClickableLabelCell
                    labels={labelsList}
                    onClick={onAgentLabelClick}
                  />
                ),
              },
              {
                altKey: 'connect-btn',
                render: server => (
                  <Cell align="right">
                    <ConnectServerActionButton server={server} />
                  </Cell>
                ),
              },
            ]}
            customSort={customSort}
            emptyText={emptyText}
            emptyHint={emptyHint}
            emptyButton={emptyButton}
            data={servers}
          />
          <SearchPagination prevPage={prevPage} nextPage={nextPage} />
        </DarkenWhileDisabled>
      </StyledTableWrapper>
    </>
  );
}

const renderAddressCell = ({ addr, tunnel }: Server) => (
  <Cell>
    {tunnel && (
      <span
        style={{ cursor: 'default', whiteSpace: 'nowrap' }}
        title="This node is connected to cluster through reverse tunnel"
      >
        ← tunnel
      </span>
    )}
    {!tunnel && addr}
  </Cell>
);
