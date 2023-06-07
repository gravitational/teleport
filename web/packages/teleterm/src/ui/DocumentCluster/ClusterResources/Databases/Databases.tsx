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
import { MenuLogin, MenuLoginProps } from 'shared/components/MenuLogin';
import { SearchPanel, SearchPagination } from 'shared/components/Search';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { IAppContext } from 'teleterm/ui/types';
import { GatewayProtocol } from 'teleterm/services/tshd/types';
import { makeDatabase } from 'teleterm/ui/services/clusters';
import { DatabaseUri } from 'teleterm/ui/uri';

import { DarkenWhileDisabled } from '../DarkenWhileDisabled';
import { getEmptyTableText } from '../getEmptyTableText';

import { useDatabases, State } from './useDatabases';

export default function Container() {
  const state = useDatabases();
  return <DatabaseList {...state} />;
}

function DatabaseList(props: State) {
  const {
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
  const dbs = fetchAttempt.data?.agentsList.map(makeDatabase) || [];
  const disabled = fetchAttempt.status === 'processing';
  const emptyText = getEmptyTableText(fetchAttempt.status, 'databases');

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
            data={dbs}
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
                  <ClickableLabelCell
                    labels={labels}
                    onClick={onAgentLabelClick}
                  />
                ),
              },
              {
                altKey: 'connect-btn',
                render: db => (
                  <ConnectButton
                    dbUri={db.uri}
                    protocol={db.protocol as GatewayProtocol}
                    onConnect={dbUser => connect(db, dbUser)}
                  />
                ),
              },
            ]}
            customSort={customSort}
            emptyText={emptyText}
          />
          <SearchPagination prevPage={prevPage} nextPage={nextPage} />
        </DarkenWhileDisabled>
      </StyledTableWrapper>
    </>
  );
}

function ConnectButton({
  dbUri,
  protocol,
  onConnect,
}: {
  dbUri: DatabaseUri;
  protocol: GatewayProtocol;
  onConnect: (dbUser: string) => void;
}) {
  const appContext = useAppContext();

  return (
    <Cell align="right">
      <MenuLogin
        {...getMenuLoginOptions(protocol)}
        width="195px"
        getLoginItems={() => getDatabaseUsers(appContext, dbUri)}
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
    </Cell>
  );
}

function getMenuLoginOptions(
  protocol: GatewayProtocol
): Pick<MenuLoginProps, 'placeholder' | 'required'> {
  if (protocol === 'redis') {
    return {
      placeholder: 'Enter username (optional)',
      required: false,
    };
  }

  return {
    placeholder: 'Enter username',
    required: true,
  };
}

async function getDatabaseUsers(appContext: IAppContext, dbUri: DatabaseUri) {
  try {
    const dbUsers = await retryWithRelogin(appContext, dbUri, () =>
      appContext.resourcesService.getDbUsers(dbUri)
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
