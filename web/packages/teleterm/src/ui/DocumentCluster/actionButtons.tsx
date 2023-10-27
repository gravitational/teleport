/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { MenuLogin, MenuLoginProps } from 'shared/components/MenuLogin';

import { ButtonBorder } from 'design';

import {
  connectToServer,
  connectToDatabase,
} from 'teleterm/ui/services/workspacesService';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import {
  Server,
  Kube,
  GatewayProtocol,
  Database,
} from 'teleterm/services/tshd/types';

import { DatabaseUri } from 'teleterm/ui/uri';
import { IAppContext } from 'teleterm/ui/types';
import { retryWithRelogin } from 'teleterm/ui/utils';

export function ConnectServerActionButton(props: {
  server: Server;
}): React.JSX.Element {
  const ctx = useAppContext();

  function getSshLogins(): string[] {
    const cluster = ctx.clustersService.findClusterByResource(props.server.uri);
    return cluster?.loggedInUser?.sshLoginsList || [];
  }

  function connect(login: string): void {
    const { uri, hostname } = props.server;
    connectToServer(
      ctx,
      { uri, hostname, login },
      {
        origin: 'resource_table',
      }
    );
  }

  return (
    <MenuLogin
      getLoginItems={() => getSshLogins().map(login => ({ login, url: '' }))}
      onSelect={(e, login) => connect(login)}
      transformOrigin={{
        vertical: 'top',
        horizontal: 'right',
      }}
      anchorOrigin={{
        vertical: 'center',
        horizontal: 'right',
      }}
    />
  );
}

export function ConnectKubeActionButton(props: {
  kube: Kube;
}): React.JSX.Element {

  function connect(): void {
    ctx.connectKube(props.kube.uri, { origin: 'resource_table' });
  }

  return (
    <ButtonBorder size="small" onClick={connect}>
      Connect
    </ButtonBorder>
  );
}

export function ConnectDatabaseActionButton(props: {
  database: Database;
}): React.JSX.Element {
  const appContext = useAppContext();

  function connect(dbUser: string): void {
    const { uri, name, protocol } = props.database;
    connectToDatabase(
      appContext,
      { uri, name, protocol, dbUser },
      { origin: 'resource_table' }
    );
  }

  return (
    <MenuLogin
      {...getDatabaseMenuLoginOptions(
        props.database.protocol as GatewayProtocol
      )}
      width="195px"
      getLoginItems={() => getDatabaseUsers(appContext, props.database.uri)}
      onSelect={(_, user) => {
        connect(user);
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
  );
}

function getDatabaseMenuLoginOptions(
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
