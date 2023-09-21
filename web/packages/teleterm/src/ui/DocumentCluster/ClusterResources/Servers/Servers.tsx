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
import { Danger } from 'design/Alert';
import { MenuLogin } from 'shared/components/MenuLogin';
import { SearchPanel, SearchPagination } from 'shared/components/Search';

import { makeServer } from 'teleterm/ui/services/clusters';

import { DarkenWhileDisabled } from '../DarkenWhileDisabled';
import { getEmptyTableText } from '../getEmptyTableText';

import { useServers, State } from './useServers';

export default function Container() {
  const state = useServers();
  return <ServerList {...state} />;
}

function ServerList(props: State) {
  const {
    getSshLogins,
    connect,
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
  const servers = fetchAttempt.data?.agentsList.map(makeServer) || [];
  const disabled = fetchAttempt.status === 'processing';
  const emptyText = getEmptyTableText(fetchAttempt.status, 'servers');

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
                key: 'labels',
                headerText: 'Labels',
                render: ({ labels }) => (
                  <ClickableLabelCell
                    labels={labels}
                    onClick={onAgentLabelClick}
                  />
                ),
              },
              {
                altKey: 'connect-btn',
                render: server =>
                  renderConnectCell(
                    () => getSshLogins(server.uri),
                    login => connect(server, login)
                  ),
              },
            ]}
            customSort={customSort}
            emptyText={emptyText}
            data={servers}
          />
          <SearchPagination prevPage={prevPage} nextPage={nextPage} />
        </DarkenWhileDisabled>
      </StyledTableWrapper>
    </>
  );
}

const renderConnectCell = (
  getSshLogins: () => string[],
  onConnect: (login: string) => void
) => {
  return (
    <Cell align="right">
      <MenuLogin
        getLoginItems={() => getSshLogins().map(login => ({ login, url: '' }))}
        onSelect={(e, login) => onConnect(login)}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        anchorOrigin={{
          vertical: 'center',
          horizontal: 'right',
        }}
      />
    </Cell>
  );
};

const renderAddressCell = ({ addr, tunnel }: ReturnType<typeof makeServer>) => (
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
