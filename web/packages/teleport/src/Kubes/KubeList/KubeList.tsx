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

import { Kube } from 'teleport/services/kube';
import { AuthType } from 'teleport/services/user';
import { ResourceLabel, ResourceFilter } from 'teleport/services/agents';
import ServersideSearchPanel from 'teleport/components/ServersideSearchPanel';

import ConnectDialog from '../ConnectDialog';

import type { PageIndicators } from 'teleport/components/hooks/useServersidePagination';

function KubeList(props: Props) {
  const {
    kubes = [],
    pageSize,
    username,
    authType,
    clusterId,
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

  const [kubeConnectName, setKubeConnectName] = useState('');

  return (
    <>
      <Table
        data={kubes}
        columns={[
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
            altKey: 'connect-btn',
            render: kube => renderConnectButtonCell(kube, setKubeConnectName),
          },
        ]}
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
        emptyText="No Kubernetes Clusters Found"
        pagination={{ pageSize }}
      />
      {kubeConnectName && (
        <ConnectDialog
          onClose={() => setKubeConnectName('')}
          username={username}
          authType={authType}
          kubeConnectName={kubeConnectName}
          clusterId={clusterId}
          accessRequestId={accessRequestId}
        />
      )}
    </>
  );
}

export const renderConnectButtonCell = (
  { name }: Kube,
  setKubeConnectName: React.Dispatch<React.SetStateAction<string>>
) => {
  return (
    <Cell align="right">
      <ButtonBorder size="small" onClick={() => setKubeConnectName(name)}>
        Connect
      </ButtonBorder>
    </Cell>
  );
};

type Props = {
  kubes: Kube[];
  pageSize: number;
  username: string;
  authType: AuthType;
  clusterId: string;
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

export default KubeList;
