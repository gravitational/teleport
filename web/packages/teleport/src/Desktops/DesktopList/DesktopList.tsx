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

import { LoginItem, MenuLogin } from 'shared/components/MenuLogin';

import { Desktop } from 'teleport/services/desktops';
import { ResourceLabel, ResourceFilter } from 'teleport/services/agents';
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
  params: ResourceFilter;
  setParams: (params: ResourceFilter) => void;
  setSort: (sort: SortType) => void;
  pathname: string;
  replaceHistory: (path: string) => void;
  onLabelClick: (label: ResourceLabel) => void;
  pageIndicators: PageIndicators;
};

export default DesktopList;
