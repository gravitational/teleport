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

import React, { useState } from 'react';
import { ButtonBorder } from 'design';
import Table, { Cell, ClickableLabelCell } from 'design/DataTable';
import { FetchStatus, SortType } from 'design/DataTable/types';
import { DbProtocol } from 'shared/services/databases';

import { AuthType } from 'teleport/services/user';
import { Database } from 'teleport/services/databases';
import { ResourceLabel, ResourceFilter } from 'teleport/services/agents';
import ConnectDialog from 'teleport/Databases/ConnectDialog';
import ServersideSearchPanel from 'teleport/components/ServersideSearchPanel';

import type { PageIndicators } from 'teleport/components/hooks/useServersidePagination';

function DatabaseList(props: Props) {
  const {
    databases = [],
    pageSize,
    username,
    clusterId,
    authType,
    fetchNext,
    fetchPrev,
    fetchStatus,
    params,
    setParams,
    setSort,
    pathname,
    replaceHistory,
    onLabelClick,
    accessRequestId,
    pageIndicators,
  } = props;

  const [dbConnectInfo, setDbConnectInfo] = useState<{
    name: string;
    protocol: DbProtocol;
  }>(null);

  return (
    <>
      <Table
        data={databases}
        columns={[
          {
            key: 'name',
            headerText: 'Name',
            isSortable: true,
          },
          {
            key: 'description',
            headerText: 'Description',
            isSortable: true,
          },
          {
            key: 'type',
            headerText: 'Type',
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
            altKey: 'connect-btn',
            render: database => renderConnectButton(database, setDbConnectInfo),
          },
        ]}
        pagination={{ pageSize }}
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
        emptyText="No Databases Found"
      />
      {dbConnectInfo && (
        <ConnectDialog
          username={username}
          clusterId={clusterId}
          dbName={dbConnectInfo.name}
          dbProtocol={dbConnectInfo.protocol}
          onClose={() => setDbConnectInfo(null)}
          authType={authType}
          accessRequestId={accessRequestId}
        />
      )}
    </>
  );
}

function renderConnectButton(
  { name, protocol }: Database,
  setDbConnectInfo: React.Dispatch<
    React.SetStateAction<{
      name: string;
      protocol: DbProtocol;
    }>
  >
) {
  return (
    <Cell align="right">
      <ButtonBorder
        size="small"
        onClick={() => {
          setDbConnectInfo({ name, protocol });
        }}
      >
        Connect
      </ButtonBorder>
    </Cell>
  );
}

type Props = {
  databases: Database[];
  pageSize: number;
  username: string;
  clusterId: string;
  authType: AuthType;
  fetchNext: () => void;
  fetchPrev: () => void;
  fetchStatus: FetchStatus;
  params: ResourceFilter;
  setParams: (params: ResourceFilter) => void;
  setSort: (sort: SortType) => void;
  pathname: string;
  replaceHistory: (path: string) => void;
  onLabelClick: (label: ResourceLabel) => void;
  accessRequestId?: string;
  pageIndicators: PageIndicators;
};

export default DatabaseList;
