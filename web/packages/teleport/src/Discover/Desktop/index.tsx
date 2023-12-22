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
