/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { ResourceViewConfig } from 'teleport/Discover/flow';
import { Finished, ResourceKind } from 'teleport/Discover/Shared';
import { DiscoverEvent } from 'teleport/services/userEvent';

import { ResourceSpec } from '../SelectResource';
import { SetupConnect } from './SetupConnect';
import { TestConnection } from './TestConnection';

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
      component: TestConnection,
      eventName: DiscoverEvent.TestConnection,
      manuallyEmitSuccessEvent: true,
    },
    {
      title: 'Finished',
      component: Finished,
      hide: true,
      eventName: DiscoverEvent.Completed,
    },
  ],
};
