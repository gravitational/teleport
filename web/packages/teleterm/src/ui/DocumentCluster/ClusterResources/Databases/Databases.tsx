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
import { useDatabases, State } from './useDatabases';
import { Table } from 'teleterm/ui/components/Table';
import { Cell } from 'design/DataTable';
import { renderLabelCell } from '../renderLabelCell';
import { Danger } from 'design/Alert';
import { MenuLogin } from 'shared/components/MenuLogin';
import { MenuLoginTheme } from '../MenuLoginTheme';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { IAppContext } from 'teleterm/ui/types';

export default function Container() {
  const state = useDatabases();
  return <DatabaseList {...state} />;
}

function DatabaseList(props: State) {
  return (
    <>
      {props.syncStatus.status === 'failed' && (
        <Danger>{props.syncStatus.statusText}</Danger>
      )}
      <Table
        data={props.dbs}
        columns={[
          {
            key: 'name',
            headerText: 'Name',
            isSortable: true,
          },
          {
            key: 'labelsList',
            headerText: 'Labels',
            render: renderLabelCell,
          },
          {
            altKey: 'connect-btn',
            render: db => (
              <ConnectButton
                documentUri={props.documentUri}
                dbUri={db.uri}
                onConnect={dbUser => props.connect(db.uri, dbUser)}
              />
            ),
          },
        ]}
        pagination={{ pageSize: 100, pagerPosition: 'bottom' }}
        emptyText="No Databases Found"
      />
    </>
  );
}

function ConnectButton({
  documentUri,
  dbUri,
  onConnect,
}: {
  documentUri: string;
  dbUri: string;
  onConnect: (dbUser: string) => void;
}) {
  const appContext = useAppContext();

  return (
    <Cell align="right">
      <MenuLoginTheme>
        <MenuLogin
          placeholder="Enter username"
          getLoginItems={() => getDatabaseUsers(appContext, documentUri, dbUri)}
          onSelect={(_, user) => {
            onConnect(user);
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'right',
          }}
          anchorOrigin={{
            vertical: 'center',
            horizontal: 'right',
          }}
        />
      </MenuLoginTheme>
    </Cell>
  );
}

async function getDatabaseUsers(
  appContext: IAppContext,
  documentUri: string,
  dbUri: string
) {
  try {
    const dbUsers = await retryWithRelogin(appContext, documentUri, dbUri, () =>
      appContext.clustersService.getDbUsers(dbUri)
    );
    return dbUsers.map(user => ({ login: user, url: '' }));
  } catch (e) {
    // Emitting a warning instead of an error here because fetching those username suggestions is
    // not the most important part of the app.
    appContext.notificationsService.notifyWarning({
      title: 'Could not fetch database usernames',
      description: e.message,
    });

    throw e;
  }
}
