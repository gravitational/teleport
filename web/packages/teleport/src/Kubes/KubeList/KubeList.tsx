/*
Copyright 2021 Gravitational, Inc.

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
import Table, { Cell, LabelCell } from 'design/DataTable';
import { ButtonBorder } from 'design';
import { Kube } from 'teleport/services/kube';
import { AuthType } from 'teleport/services/user';
import ConnectDialog from '../ConnectDialog';

function KubeList(props: Props) {
  const { kubes = [], pageSize = 100, username, authType, clusterId } = props;

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
            key: 'tags',
            headerText: 'Labels',
            render: ({ tags }) => <LabelCell data={tags} />,
          },
          {
            altKey: 'connect-btn',
            render: kube => renderConnectButtonCell(kube, setKubeConnectName),
          },
        ]}
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
  pageSize?: number;
  username: string;
  authType: AuthType;
  clusterId: string;
};

export default KubeList;
