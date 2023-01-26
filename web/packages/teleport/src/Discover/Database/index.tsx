/**
 * Copyright 2022 Gravitational, Inc.
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

import { Database as DatabaseIcon } from 'design/Icon';

import { ResourceKind, Finished } from 'teleport/Discover/Shared';
import { Resource } from 'teleport/Discover/flow';
import { DatabaseWrapper } from 'teleport/Discover/Database/DatabaseWrapper';
import {
  Database,
  DatabaseLocation,
} from 'teleport/Discover/Database/resources';

import { CreateDatabase } from 'teleport/Discover/Database/CreateDatabase';
import { SetupAccess } from 'teleport/Discover/Database/SetupAccess';
import { DownloadScript } from 'teleport/Discover/Database/DownloadScript';
import { MutualTls } from 'teleport/Discover/Database/MutualTls';
import { TestConnection } from 'teleport/Discover/Database/TestConnection';
import { IamPolicy } from 'teleport/Discover/Database/IamPolicy';

export const DatabaseResource: Resource<Database> = {
  kind: ResourceKind.Database,
  icon: <DatabaseIcon />,
  wrapper(component: React.ReactNode) {
    return <DatabaseWrapper>{component}</DatabaseWrapper>;
  },
  shouldPrompt(currentStep) {
    // do not prompt on exit if they're selecting a resource
    return currentStep !== 0;
  },
  views(database) {
    let configureResourceViews;
    if (database) {
      switch (database.location) {
        case DatabaseLocation.AWS:
          configureResourceViews = [
            {
              title: 'Register a Database',
              component: CreateDatabase,
            },
            {
              title: 'Deploy Database Service',
              component: DownloadScript,
            },
            {
              title: 'Configure IAM Policy',
              component: IamPolicy,
            },
          ];

          break;

        case DatabaseLocation.SelfHosted:
          configureResourceViews = [
            {
              title: 'Register a Database',
              component: CreateDatabase,
            },
            {
              title: 'Deploy Database Service',
              component: DownloadScript,
            },
            {
              title: 'Configure mTLS',
              component: MutualTls,
            },
          ];

          break;
      }
    }

    return [
      {
        title: 'Select Resource Type',
      },
      {
        title: 'Configure Resource',
        views: configureResourceViews,
      },
      {
        title: 'Set Up Access',
        component: SetupAccess,
      },
      {
        title: 'Test Connection',
        component: TestConnection,
      },
      {
        title: 'Finished',
        component: Finished,
        hide: true,
      },
    ];
  },
};
