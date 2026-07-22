/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React from 'react';

import Table, { Cell, ClickableLabelCell } from 'design/DataTable';
import { FetchStatus, SortType } from 'design/DataTable/types';
import {
  LoginItem,
  MenuInputType,
  MenuLogin,
} from 'shared/components/MenuLogin';

import type { PageIndicators } from 'teleport/components/hooks/useServersidePagination';
import { ServersideSearchPanelWithPageIndicator } from 'teleport/components/ServersideSearchPanel';
import { ResourceFilter, ResourceLabel } from 'teleport/services/agents';
import { Node } from 'teleport/services/nodes';

export function NodeList(props: {
  nodes: Node[];
  onLoginMenuOpen(serverId: string): { login: string; url: string }[];
  onLoginSelect(e: React.SyntheticEvent, login: string, serverId: string): void;
  fetchNext: () => void;
  fetchPrev: () => void;
  fetchStatus: FetchStatus;
  pageSize?: number;
  params: ResourceFilter;
  setParams: (params: ResourceFilter) => void;
  setSort: (sort: SortType) => void;
  onLabelClick: (label: ResourceLabel) => void;
  pageIndicators: PageIndicators;
}) {
  const {
    nodes = [],
    onLoginMenuOpen,
    onLoginSelect,
    pageSize,
    fetchNext,
    fetchPrev,
    fetchStatus,
    params,
    setParams,
    setSort,
    onLabelClick,
    pageIndicators,
  } = props;

  return (
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
          render: renderAddressCell,
        },
        {
          key: 'labels',
          headerText: 'Labels',
          render: ({ labels }) => (
            <ClickableLabelCell labels={labels} onClick={onLabelClick} />
          ),
        },
        {
          altKey: 'connect-btn',
          render: ({ id }) =>
            renderLoginCell(id, onLoginSelect, onLoginMenuOpen),
        },
      ]}
      emptyText="No Nodes Found"
      data={nodes}
      pagination={{
        pageSize,
      }}
      fetching={{
        onFetchNext: fetchNext,
        onFetchPrev: fetchPrev,
        fetchStatus,
      }}
      serversideProps={{
        sort: params.sort,
        setSort,
        serversideSearchPanel: (
          <ServersideSearchPanelWithPageIndicator
            pageIndicators={pageIndicators}
            params={params}
            setParams={setParams}
            disabled={fetchStatus === 'loading'}
          />
        ),
      }}
    />
  );
}

const renderLoginCell = (
  id: string,
  onSelect: (e: React.SyntheticEvent, login: string, serverId: string) => void,
  onOpen: (serverId: string) => LoginItem[]
) => {
  function handleOnOpen() {
    return onOpen(id);
  }

  function handleOnSelect(e: React.SyntheticEvent, login: string) {
    if (!onSelect) {
      return [];
    }

    return onSelect(e, login, id);
  }

  return (
    <Cell align="right">
      <MenuLogin
        inputType={MenuInputType.FILTER}
        getLoginItems={handleOnOpen}
        onSelect={handleOnSelect}
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

const renderAddressCell = ({ addr, tunnel }: Node) => (
  <Cell>{tunnel ? renderTunnel() : addr}</Cell>
);

function renderTunnel() {
  return (
    <span
      style={{ cursor: 'default', whiteSpace: 'nowrap' }}
      title="This node is connected to cluster through reverse tunnel"
    >
      ← tunnel
    </span>
  );
}
