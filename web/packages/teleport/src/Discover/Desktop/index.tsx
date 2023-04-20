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

import { ConnectTeleport } from 'teleport/Discover/Desktop/ConnectTeleport';
import { DiscoverDesktops } from 'teleport/Discover/Desktop/DiscoverDesktops';
import { InstallActiveDirectory } from 'teleport/Discover/Desktop/InstallActiveDirectory';

import { ResourceViewConfig } from 'teleport/Discover/flow';

import { DesktopWrapper } from 'teleport/Discover/Desktop/DesktopWrapper';
import { DiscoverEvent } from 'teleport/services/userEvent';

export const DesktopResource: ResourceViewConfig = {
  kind: ResourceKind.Desktop,
  wrapper: (component: React.ReactNode) => (
    <DesktopWrapper>{component}</DesktopWrapper>
  ),
  shouldPrompt(currentStep) {
    // prompt them up if they try exiting before having finished the ConnectTeleport step
    return currentStep < 3;
  },
  views: [
    {
      title: 'Install Active Directory',
      component: InstallActiveDirectory,
      eventName: DiscoverEvent.DesktopActiveDirectoryToolsInstall,
    },
    {
      title: 'Connect Teleport',
      component: ConnectTeleport,
      // Sub-step events are handled inside its component.
      // This eventName defines the event to emit for the `last` sub-step.
      eventName: DiscoverEvent.DeployService,
    },
    {
      title: 'Discover Desktops',
      component: DiscoverDesktops,
      eventName: DiscoverEvent.AutoDiscoveredResources,
      manuallyEmitSuccessEvent: true,
    },
    {
      title: 'Finished',
      component: Finished,
      eventName: DiscoverEvent.Completed,
      hide: true,
    },
  ],
};
