/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import Flex from 'design/Flex';

import AppContextProvider from 'teleterm/ui/appContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { getEmptyPendingAccessRequest } from 'teleterm/ui/services/workspacesService/accessRequestsService';
import * as types from 'teleterm/services/tshd/types';
import {
  SuggestionCmd,
  SuggestionDatabase,
  SuggestionServer,
  SuggestionSshLogin,
} from 'teleterm/ui/services/quickInput';

import QuickInput from './QuickInput';
import QuickInputList from './QuickInputList';

export default {
  title: 'Teleterm/QuickInput',
};

export const Story = () => {
  const appContext = new MockAppContext();

  appContext.workspacesService.state = {
    workspaces: {
      '/clusters/localhost': {
        documents: [],
        location: undefined,
        localClusterUri: '/clusters/localhost',
        accessRequests: {
          pending: getEmptyPendingAccessRequest(),
          isBarCollapsed: true,
        },
      },
    },
    rootClusterUri: '/clusters/localhost',
  };

  appContext.clustersService.getClusters = () => {
    return [cluster];
  };

  appContext.clustersService.setState(draftState => {
    draftState.clusters = new Map([[cluster.uri, cluster]]);
  });

  appContext.resourcesService.fetchServers = async () => ({
    agentsList: servers,
    totalCount: 3,
    startKey: '',
  });

  appContext.resourcesService.fetchDatabases = async () => ({
    agentsList: databases,
    totalCount: 3,
    startKey: '',
  });

  return (
    <AppContextProvider value={appContext}>
      <div
        css={`
          height: 40px;
        `}
      >
        <QuickInput />
      </div>
    </AppContextProvider>
  );
};

export const Suggestions = () => {
  const commandSuggestions: SuggestionCmd[] = [
    {
      kind: 'suggestion.cmd',
      token: '',
      data: {
        displayName: 'tsh foo',
        description: 'Nulla convallis lorem ut ipsum maximus venenatis.',
      },
    },
    {
      kind: 'suggestion.cmd',
      token: '',
      data: {
        displayName: 'tsh bar',
        description: 'Vivamus id nulla sed neque efficitur ornare nec in diam.',
      },
    },
    {
      kind: 'suggestion.cmd',
      token: '',
      data: {
        displayName: 'tsh quux foo',
        description:
          'Sed porta nibh eget lacus suscipit vehicula. Curabitur eget sapien in lacus blandit pretium.',
      },
    },
    {
      kind: 'suggestion.cmd',
      token: '',
      data: {
        displayName: 'tsh baz quux',
        description: 'Etiam cursus magna at feugiat ornare.',
      },
    },
  ];

  const loginSuggestions: SuggestionSshLogin[] =
    cluster.loggedInUser.sshLoginsList.map(login => ({
      kind: 'suggestion.ssh-login',
      token: '',
      appendToToken: '',
      data: login,
    }));

  const serverSuggestions: SuggestionServer[] = servers.map(server => ({
    kind: 'suggestion.server',
    token: '',
    data: server,
  }));

  const dbSuggestions: SuggestionDatabase[] = databases.map(db => ({
    kind: 'suggestion.database',
    token: '',
    data: db,
  }));

  return (
    <Flex flexWrap="wrap" p={2} gap={2}>
      <QuickInputListWrapper
        items={commandSuggestions}
        width={defaultWidth * 2}
      />
      <QuickInputListWrapper items={loginSuggestions} />
      <QuickInputListWrapper
        items={serverSuggestions}
        width={defaultWidth * 3}
        height={defaultHeight * 1.5}
      />
      <QuickInputListWrapper
        items={dbSuggestions}
        width={defaultWidth * 3}
        height={defaultHeight * 1.5}
      />
    </Flex>
  );
};

const defaultWidth = 200;
const defaultHeight = 200;

const QuickInputListWrapper = ({
  items,
  width = defaultWidth,
  height = defaultHeight,
}) => {
  return (
    <div
      css={`
        position: relative;
        width: ${width}px;
        height: ${height}px;
      `}
    >
      <QuickInputList
        items={items}
        activeItem={0}
        position={0}
        onPick={() => {}}
      />
    </div>
  );
};

const longIdentifier =
  'lorem-ipsum-dolor-sit-amet-consectetur-adipiscing-elit-quisque-elementum-nulla';

const servers: types.Server[] = [
  {
    uri: '/clusters/localhost/servers/foo' as const,
    tunnel: false,
    name: '2018454d-ef3b-4b15-84f7-61ca213d37e3',
    hostname: 'foo',
    addr: 'foo.localhost',
    labelsList: [
      { name: 'env', value: 'prod' },
      { name: 'kernel', value: '5.15.0-1023-aws' },
    ],
  },
  {
    uri: '/clusters/localhost/servers/bar' as const,
    tunnel: false,
    name: '24c7aebe-4741-4464-ab69-f076fe467ebd',
    hostname: 'bar',
    addr: 'bar.localhost',
    labelsList: [
      { name: 'env', value: 'staging' },
      { name: 'kernel', value: '5.14.1-1058-aws' },
    ],
  },
  {
    uri: '/clusters/localhost/servers/lorem' as const,
    tunnel: false,
    name: '24c7aebe-4741-4464-ab69-f076fe467ebd',
    hostname: longIdentifier,
    addr: 'lorem.localhost',
    labelsList: [
      { name: 'env', value: 'staging' },
      { name: 'kernel', value: '5.14.1-1058-aws' },
      { name: 'lorem', value: longIdentifier },
      { name: 'kernel2', value: '5.14.1-1058-aws' },
      { name: 'env2', value: 'staging' },
      { name: 'kernel3', value: '5.14.1-1058-aws' },
    ],
  },
];

const databases: types.Database[] = [
  {
    uri: '/clusters/localhost/dbs/postgres' as const,
    name: 'postgres',
    desc: 'A PostgreSQL database',
    protocol: 'postgres',
    type: 'self-hosted',
    hostname: 'postgres.localhost',
    addr: 'postgres.localhost',
    labelsList: [
      { name: 'env', value: 'prod' },
      { name: 'kernel', value: '5.15.0-1023-aws' },
    ],
  },
  {
    uri: '/clusters/localhost/dbs/mysql' as const,
    name: 'mysql',
    desc: 'A MySQL database',
    protocol: 'mysql',
    type: 'self-hosted',
    hostname: 'mysql.localhost',
    addr: 'mysql.localhost',
    labelsList: [
      { name: 'env', value: 'staging' },
      { name: 'kernel', value: '5.14.1-1058-aws' },
    ],
  },
  {
    uri: '/clusters/localhost/dbs/lorem' as const,
    name: longIdentifier,
    desc: 'Vestibulum ut blandit est, sed dapibus sem. Pellentesque egestas mi eu scelerisque ultricies.',
    protocol: 'mysql',
    type: 'self-hosted',
    hostname: 'lorem.localhost',
    addr: 'lorem.localhost',
    labelsList: [
      { name: 'env', value: 'staging' },
      { name: 'kernel', value: '5.14.1-1058-aws' },
      { name: 'lorem', value: longIdentifier },
      { name: 'kernel2', value: '5.14.1-1058-aws' },
      { name: 'env2', value: 'staging' },
      { name: 'kernel3', value: '5.14.1-1058-aws' },
    ],
  },
];

const cluster = {
  uri: '/clusters/localhost' as const,
  name: 'Test',
  leaf: false,
  connected: true,
  proxyHost: 'localhost:3080',
  authClusterId: '73c4746b-d956-4f16-9848-4e3469f70762',
  loggedInUser: {
    activeRequestsList: [],
    name: 'admin',
    acl: {},
    sshLoginsList: ['root', 'ubuntu', 'ansible', longIdentifier],
    rolesList: [],
    requestableRolesList: [],
    suggestedReviewersList: [],
  },
};
