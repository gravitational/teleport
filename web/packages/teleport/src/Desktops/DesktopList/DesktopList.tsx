/*
Copyright 2021-2022 Gravitational, Inc.

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
import Table, { Cell, ClickableLabelCell } from 'design/DataTable';
import { FetchStatus, SortType } from 'design/DataTable/types';

import { LoginItem, MenuLogin } from 'shared/components/MenuLogin';

import { Desktop } from 'teleport/services/desktops';
import { AgentLabel, AgentFilter } from 'teleport/services/agents';
import ServersideSearchPanel from 'teleport/components/ServersideSearchPanel';

import type { PageIndicators } from 'teleport/components/hooks/useServersidePagination';

function DesktopList(props: Props) {
  const {
    desktops = [],
    pageSize,
    onLoginMenuOpen,
    onLoginSelect,
    fetchNext,
    fetchPrev,
    fetchStatus,
    params,
    setParams,
    setSort,
    pathname,
    replaceHistory,
    onLabelClick,
    pageIndicators,
  } = props;

  function onDesktopSelect(
    e: React.MouseEvent,
    username: string,
    desktopName: string
  ) {
    e.preventDefault();
    onLoginSelect(username, desktopName);
  }

  return (
    <Table
      data={desktops}
      columns={[
        {
          key: 'addr',
          headerText: 'Address',
        },
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
        },
        {
          key: 'labels',
          headerText: 'Labels',
          render: ({ labels }) => (
            <ClickableLabelCell labels={labels} onClick={onLabelClick} />
          ),
        },
        {
          altKey: 'login-cell',
          render: desktop =>
            renderLoginCell(desktop, onLoginMenuOpen, onDesktopSelect),
        },
      ]}
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
          <ServersideSearchPanel
            pageIndicators={pageIndicators}
            params={params}
            setParams={setParams}
            pathname={pathname}
            replaceHistory={replaceHistory}
            disabled={fetchStatus === 'loading'}
          />
        ),
      }}
      isSearchable
      emptyText="No Desktops Found"
    />
  );
}

function renderLoginCell(
  desktop: Desktop,
  onOpen: (desktop: Desktop) => LoginItem[],
  onSelect: (
    e: React.SyntheticEvent,
    username: string,
    desktopName: string
  ) => void
) {
  function handleOnOpen() {
    return onOpen(desktop);
  }

  function handleOnSelect(e: React.SyntheticEvent, login: string) {
    if (!onSelect) {
      return [];
    }

    return onSelect(e, login, desktop.name);
  }

  return (
    <Cell align="right">
      <MenuLogin
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
}

type Props = {
  desktops: Desktop[];
  pageSize: number;
  username: string;
  clusterId: string;
  onLoginMenuOpen(desktop: Desktop): LoginItem[];
  onLoginSelect(username: string, desktopName: string): void;
  fetchNext: () => void;
  fetchPrev: () => void;
  fetchStatus: FetchStatus;
  params: AgentFilter;
  setParams: (params: AgentFilter) => void;
  setSort: (sort: SortType) => void;
  pathname: string;
  replaceHistory: (path: string) => void;
  onLabelClick: (label: AgentLabel) => void;
  pageIndicators: PageIndicators;
};

export default DesktopList;
