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

import { ResourceViewConfig } from 'teleport/Discover/flow';
import { DownloadScript } from 'teleport/Discover/Server/DownloadScript';
import { SetupAccess } from 'teleport/Discover/Server/SetupAccess';
import { TestConnection } from 'teleport/Discover/Server/TestConnection';
import { AwsAccount, ResourceKind, Finished } from 'teleport/Discover/Shared';
import { DiscoverEvent } from 'teleport/services/userEvent';

import { ResourceSpec, ServerLocation } from '../SelectResource';

import { EnrollEc2Instance } from './EnrollEc2Instance/EnrollEc2Instance';
import { CreateEc2Ice } from './CreateEc2Ice/CreateEc2Ice';

import { ServerWrapper } from './ServerWrapper';

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
    if (resource && resource.nodeMeta?.location === ServerLocation.Aws) {
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
