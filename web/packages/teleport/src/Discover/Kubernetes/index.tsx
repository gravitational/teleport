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

import React from 'react';

import { ResourceViewConfig } from 'teleport/Discover/flow';
import { EnrollEksCluster } from 'teleport/Discover/Kubernetes/EnrollEKSCluster';
import { KubeLocation, ResourceSpec } from 'teleport/Discover/SelectResource';
import { AwsAccount, Finished, ResourceKind } from 'teleport/Discover/Shared';
import { DiscoverEvent } from 'teleport/services/userEvent';

import { KubeWrapper } from './KubeWrapper';
import { HelmChart } from './SelfHosted';
import { SetupAccess } from './SetupAccess';
import { TestConnection } from './TestConnection';

export const KubernetesResource: ResourceViewConfig = {
  kind: ResourceKind.Kubernetes,
  wrapper: (component: React.ReactNode) => (
    <KubeWrapper>{component}</KubeWrapper>
  ),
  shouldPrompt(currentStep, currentView, resourceSpec) {
    if (resourceSpec?.kubeMeta?.location === KubeLocation.Aws) {
      // Allow user to bypass prompting on this step (Connect AWS Account)
      // on exit because users might need to change route to setup an
      // integration.
      if (currentStep === 0) {
        return false;
      }
    }
    return currentView?.eventName !== DiscoverEvent.Completed;
  },
  views(resource: ResourceSpec) {
    let configuredResourceViews = [
      {
        title: 'Configure Resource',
        component: HelmChart,
        eventName: DiscoverEvent.DeployService,
      },
    ];
    if (resource?.kubeMeta?.location === KubeLocation.Aws) {
      configuredResourceViews = [
        {
          title: 'Connect AWS Account',
          component: AwsAccount,
          eventName: DiscoverEvent.IntegrationAWSOIDCConnectEvent,
        },
        {
          title: 'Enroll EKS Clusters',
          component: EnrollEksCluster,
          eventName: DiscoverEvent.KubeEKSEnrollEvent,
        },
      ];
    }

    return [
      ...configuredResourceViews,
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
    ];
  },
};
