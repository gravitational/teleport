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

import { Finished, ResourceKind } from 'teleport/Discover/Shared';
import { ResourceViewConfig } from 'teleport/Discover/flow';
import { DiscoverEvent } from 'teleport/services/userEvent';

import { KubeWrapper } from './KubeWrapper';
import { SetupAccess } from './SetupAccess';
import { HelmChart } from './HelmChart';
import { TestConnection } from './TestConnection';

export const KubernetesResource: ResourceViewConfig = {
  kind: ResourceKind.Kubernetes,
  wrapper: (component: React.ReactNode) => (
    <KubeWrapper>{component}</KubeWrapper>
  ),
  views: [
    {
      title: 'Configure Resource',
      component: HelmChart,
      eventName: DiscoverEvent.DeployService,
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
      hide: true,
      eventName: DiscoverEvent.Completed,
    },
  ],
};
