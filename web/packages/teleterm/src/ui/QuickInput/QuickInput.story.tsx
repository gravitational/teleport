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

import AppContextProvider from 'teleterm/ui/appContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import QuickInput from './QuickInput';

export default {
  title: 'Teleterm/QuickInput',
};

export const Story = () => {
  const appContext = new MockAppContext();

  appContext.workspacesService.state = {
    workspaces: {
      '/clusters/localhost': {
        documents: [],
        location: '',
        localClusterUri: '/clusters/localhost',
      },
    },
    rootClusterUri: '/clusters/localhost',
  };

  appContext.clustersService.getClusters = () => {
    return [
      {
        uri: '/clusters/localhost',
        name: 'Test',
        leaf: false,
        connected: true,
        proxyHost: 'localhost:3080',
        loggedInUser: {
          name: 'admin',
          acl: {},
          sshLoginsList: [],
          rolesList: [],
        },
      },
    ];
  };

  return (
    <AppContextProvider value={appContext}>
      <QuickInput />
    </AppContextProvider>
  );
};
