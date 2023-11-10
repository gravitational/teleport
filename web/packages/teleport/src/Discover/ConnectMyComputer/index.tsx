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

import { ResourceViewConfig } from 'teleport/Discover/flow';
import { ResourceKind } from 'teleport/Discover/Shared';
import { DiscoverEvent } from 'teleport/services/userEvent';

import { ResourceSpec } from '../SelectResource';

import { SetupConnect } from './SetupConnect';

export const ConnectMyComputerResource: ResourceViewConfig<ResourceSpec> = {
  kind: ResourceKind.ConnectMyComputer,
  views: [
    {
      title: 'Set Up Teleport Connect',
      component: SetupConnect,
      eventName: DiscoverEvent.DeployService,
    },
    {
      title: 'Test Connection',
      component: () => <div>WIP</div>,
      eventName: DiscoverEvent.TestConnection,
      // TODO(ravicious): Manually emit success event on test connection success.
      // manuallyEmitSuccessEvent: true,
    },
  ],
};
