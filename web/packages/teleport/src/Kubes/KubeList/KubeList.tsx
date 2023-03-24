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

import React, { useState } from 'react';
import { ButtonBorder } from 'design';
import Table, { Cell, ClickableLabelCell } from 'design/DataTable';
import { FetchStatus, SortType } from 'design/DataTable/types';

import { Kube } from 'teleport/services/kube';
import { AuthType } from 'teleport/services/user';
import { AgentLabel, AgentFilter } from 'teleport/services/agents';
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
  params: AgentFilter;
  setParams: (params: AgentFilter) => void;
  setSort: (sort: SortType) => void;
  pathname: string;
  replaceHistory: (path: string) => void;
  onLabelClick: (label: AgentLabel) => void;
  accessRequestId?: string;
  pageIndicators: PageIndicators;
};

export default KubeList;
