/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { AwsAccount, ResourceKind, Finished } from 'teleport/Discover/Shared';
import { ResourceViewConfig } from 'teleport/Discover/flow';
import { ResourceSpec } from 'teleport/Discover/SelectResource';

import { DiscoverEvent } from 'teleport/services/userEvent';

import { CreateAppAccess } from './CreateAppAccess/CreateAppAccess';
import { SetupAccess } from './SetupAccess/SetupAccess';
import { TestConnection } from './TestConnection/TestConnection';

export const AwsMangementConsole: ResourceViewConfig<ResourceSpec> = {
  kind: ResourceKind.Application,
  shouldPrompt(currentStep) {
    return currentStep !== 0;
  },
  views() {
    return [
      {
        title: 'Connect AWS Account',
        component: AwsAccount,
        eventName: DiscoverEvent.IntegrationAWSOIDCConnectEvent,
      },
      {
        title: 'Create Applicaton Server',
        component: CreateAppAccess,
        // TODO(lisa) define a create application aws step
      },
      {
        title: 'Set Up Access',
        component: SetupAccess,
        eventName: DiscoverEvent.PrincipalsConfigure,
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
        eventName: DiscoverEvent.Completed,
        hide: true,
      },
    ];
  },
};
