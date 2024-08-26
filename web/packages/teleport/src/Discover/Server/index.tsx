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
import { DownloadScript } from 'teleport/Discover/Server/DownloadScript';
import { SetupAccess } from 'teleport/Discover/Server/SetupAccess';
import { TestConnection } from 'teleport/Discover/Server/TestConnection';
import { AwsAccount, ResourceKind, Finished } from 'teleport/Discover/Shared';
import {
  DiscoverDiscoveryConfigMethod,
  DiscoverEvent,
} from 'teleport/services/userEvent';
import cfg from 'teleport/config';

import { ResourceSpec, ServerLocation } from '../SelectResource';
import { ConfigureDiscoveryService } from '../Shared/ConfigureDiscoveryService';

import { EnrollEc2Instance } from './EnrollEc2Instance/EnrollEc2Instance';
import { CreateEc2Ice } from './CreateEc2Ice/CreateEc2Ice';

import { ServerWrapper } from './ServerWrapper';
import { DiscoveryConfigSsm } from './DiscoveryConfigSsm/DiscoveryConfigSsm';

export const ServerResource: ResourceViewConfig<ResourceSpec> = {
  kind: ResourceKind.Server,
  wrapper: (component: React.ReactNode) => (
    <ServerWrapper>{component}</ServerWrapper>
  ),
  shouldPrompt(currentStep, resourceSpec) {
    if (resourceSpec?.nodeMeta?.location === ServerLocation.Aws) {
      // Allow user to bypass prompting on this step (Connect AWS Connect)
      // on exit because users might need to change route to setup an
      // integration.
      if (currentStep === 0) {
        return false;
      }
    }
    return true;
  },

  views(resource) {
    let configureResourceViews;
    const { nodeMeta } = resource;
    if (
      nodeMeta?.location === ServerLocation.Aws &&
      nodeMeta.discoveryConfigMethod ===
        DiscoverDiscoveryConfigMethod.AwsEc2Eice
    ) {
      configureResourceViews = [
        {
          title: 'Connect AWS Account',
          component: AwsAccount,
          eventName: DiscoverEvent.IntegrationAWSOIDCConnectEvent,
        },
        {
          title: 'Enroll EC2 Instance',
          component: EnrollEc2Instance,
          eventName: DiscoverEvent.EC2InstanceSelection,
        },
        {
          title: 'Create EC2 Instance Connect Endpoint',
          component: CreateEc2Ice,
          eventName: DiscoverEvent.CreateNode,
          manuallyEmitSuccessEvent: true,
        },
      ];
    } else if (
      nodeMeta?.location === ServerLocation.Aws &&
      nodeMeta.discoveryConfigMethod === DiscoverDiscoveryConfigMethod.AwsEc2Ssm
    ) {
      configureResourceViews = [
        {
          title: 'Connect AWS Account',
          component: AwsAccount,
          eventName: DiscoverEvent.IntegrationAWSOIDCConnectEvent,
        },
        // Self hosted requires user to manually install a discovery service.
        // Cloud already has a discovery service running, so this step is not required.
        ...(!cfg.isCloud
          ? [
              {
                title: 'Configure Discovery Service',
                component: ConfigureDiscoveryService,
                eventName: DiscoverEvent.DeployService,
              },
            ]
          : []),
        {
          title: cfg.isCloud
            ? 'Configure Auto Discovery Service'
            : 'Create Discovery Config',
          component: DiscoveryConfigSsm,
          eventName: DiscoverEvent.CreateDiscoveryConfig,
        },
      ];
    } else {
      configureResourceViews = [
        {
          title: 'Configure Resource',
          component: DownloadScript,
          eventName: DiscoverEvent.DeployService,
        },
      ];
    }

    return [
      {
        title: 'Configure Resource',
        views: configureResourceViews,
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
    ];
  },
};
