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
import { ButtonBorder } from 'design';
import Table, { Cell, LabelCell } from 'design/DataTableNext';
import { AuthType } from 'teleport/services/user';
import { Database, DbProtocol } from 'teleport/services/databases';
import ConnectDialog from 'teleport/Databases/ConnectDialog';

function DatabaseList(props: Props) {
  const {
    databases = [],
    pageSize = 100,
    username,
    clusterId,
    authType,
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
            key: 'desc',
            headerText: 'Description',
            isSortable: true,
          },
          {
            key: 'title',
            headerText: 'Type',
            isSortable: true,
          },
          {
            key: 'tags',
            headerText: 'Labels',
            render: ({ tags }) => <LabelCell data={tags} />,
          },
          {
            altKey: 'connect-btn',
            render: database => renderConnectButton(database, setDbConnectInfo),
          },
        ]}
        pagination={{ pageSize }}
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
  pageSize?: number;
  username: string;
  clusterId: string;
  authType: AuthType;
};

export default DatabaseList;
