/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { Meta, StoryObj } from '@storybook/react-vite';
import { useEffect, useState } from 'react';

import { Flex, H3, P2, Stack } from 'design';
import {
  ClusterVersionInfo,
  UnreachableCluster,
} from 'gen-proto-ts/teleport/lib/teleterm/auto_update/v1/auto_update_service_pb';

import { Platform } from 'teleterm/mainProcess/types';
import { AppUpdateEvent } from 'teleterm/services/appUpdater';
import { resolveAutoUpdatesStatus } from 'teleterm/services/appUpdater/autoUpdatesStatus';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import { DetailsView } from './DetailsView';
import {
  makeCheckingForUpdateEvent,
  makeDownloadProgressEvent,
  makeErrorEvent,
  makeUpdateAvailableEvent,
  makeUpdateDownloadedEvent,
  makeUpdateInfo,
  makeUpdateNotAvailableEvent,
} from './testHelpers';
import { WidgetView } from './WidgetView';

export interface StoryProps {
  step:
    | 'Update not available'
    | 'Checking for update'
    | 'Update available'
    | 'Download progress'
    | 'Error'
    | 'Update downloaded';
  updateSource: string;
  envVar: 'Set to "off"' | 'Set to version - v15' | 'Unset';
  platform: Platform;
  clusterFoo:
    | 'Does not exist'
    | 'Unreachable'
    | 'Enabled client updates - v18 cluster'
    | 'Enabled client updates - v17 cluster'
    | 'Disabled client updates - v18 cluster';
  clusterBar:
    | 'Does not exist'
    | 'Unreachable'
    | 'Enabled client updates - v17 cluster'
    | 'Enabled client updates - v16 cluster'
    | 'Disabled client updates - v17 cluster';
  clusterBarSetToManageUpdates: boolean;
  updateKind: 'Upgrade' | 'Downgrade';
  nonTeleportCdn: boolean;
}

const meta: Meta<StoryProps> = {
  title: 'Teleterm/AppUpdater',
  component: WidgetAndDetails,
  argTypes: {
    envVar: {
      control: { type: 'radio' },
      options: ['Off', 'Set to version - v15', 'Unset'],
      description: '`TELEPORT_TOOLS_VERSION` value',
    },
    clusterFoo: {
      control: { type: 'select' },
      description: 'State of cluster "foo"',
      options: [
        'Does not exist',
        'Enabled client updates - v18 cluster',
        'Enabled client updates - v17 cluster',
        'Disabled client updates - v18 cluster',
        'Unreachable',
      ],
    },
    clusterBar: {
      description: 'State of cluster "bar"',
      control: { type: 'select' },
      options: [
        'Does not exist',
        'Enabled client updates - v17 cluster',
        'Enabled client updates - v16 cluster',
        'Disabled client updates - v17 cluster',
        'Unreachable',
      ],
    },
    clusterBarSetToManageUpdates: {
      control: { type: 'boolean' },
      description: 'Whether cluster "bar" is manually set to control updates',
    },
    updateKind: {
      control: { type: 'radio' },
      options: ['Upgrade', 'Downgrade'],
      description:
        'Indicates whether the update version is newer or older than the current application version.',
    },
    step: {
      control: { type: 'radio' },
      options: [
        'Update not available',
        'Checking for update',
        'Update available',
        'Download progress',
        'Error',
        'Update downloaded',
      ],
      description: 'Updating process step',
    },
    platform: {
      control: { type: 'radio' },
      options: ['win32', 'darwin', 'linux'],
      description: 'Operating system',
    },
    nonTeleportCdn: {
      control: { type: 'boolean' },
      description:
        'Whether `TELEPORT_CDN_BASE_URL` is set to non-Teleport CDN URL',
    },
  },
  args: {
    envVar: 'Unset',
    clusterFoo: 'Enabled client updates - v18 cluster',
    clusterBar: 'Does not exist',
    clusterBarSetToManageUpdates: false,
    updateKind: 'Upgrade',
    step: 'Update available',
    platform: 'darwin',
    nonTeleportCdn: false,
  },
};

export default meta;

const context = new MockAppContext();
context.addRootCluster(makeRootCluster({ uri: '/clusters/foo', name: 'foo' }));
context.addRootCluster(makeRootCluster({ uri: '/clusters/bar', name: 'bar' }));

async function resolveEvent(storyProps: StoryProps): Promise<AppUpdateEvent> {
  const status = await resolveAutoUpdatesStatus({
    versionEnvVar:
      storyProps.envVar === 'Set to version - v15'
        ? '15.0.0'
        : storyProps.envVar === 'Unset'
          ? undefined
          : 'off',
    managingClusterUri: storyProps.clusterBarSetToManageUpdates
      ? '/clusters/bar'
      : undefined,
    getClusterVersions: async () => {
      const reachableClusters: ClusterVersionInfo[] = [];
      const unreachableClusters: UnreachableCluster[] = [];

      switch (storyProps.clusterFoo) {
        case 'Does not exist':
          break;
        case 'Enabled client updates - v18 cluster':
          reachableClusters.push({
            toolsVersion: '18.0.0',
            minToolsVersion: '17.0.0-aa',
            clusterUri: '/clusters/foo',
            toolsAutoUpdate: true,
          });
          break;
        case 'Enabled client updates - v17 cluster':
          reachableClusters.push({
            toolsVersion: '17.0.0',
            minToolsVersion: '16.0.0-aa',
            clusterUri: '/clusters/foo',
            toolsAutoUpdate: true,
          });
          break;
        case 'Disabled client updates - v18 cluster':
          reachableClusters.push({
            toolsVersion: '18.0.0',
            minToolsVersion: '17.0.0-aa',
            clusterUri: '/clusters/foo',
            toolsAutoUpdate: false,
          });
          break;
        case 'Unreachable':
          unreachableClusters.push({
            clusterUri: '/clusters/foo',
            errorMessage:
              'transport: Error while dialing: failed to dial: dial tcp\n' +
              '192.168.100.39:3080: connect: connection refused',
          });
      }
      switch (storyProps.clusterBar) {
        case 'Does not exist':
          break;
        case 'Enabled client updates - v17 cluster':
          reachableClusters.push({
            toolsVersion: '17.0.0',
            minToolsVersion: '16.0.0-aa',
            clusterUri: '/clusters/bar',
            toolsAutoUpdate: true,
          });
          break;
        case 'Enabled client updates - v16 cluster':
          reachableClusters.push({
            toolsVersion: '16.0.0',
            minToolsVersion: '15.0.0-aa',
            clusterUri: '/clusters/bar',
            toolsAutoUpdate: true,
          });
          break;
        case 'Disabled client updates - v17 cluster':
          reachableClusters.push({
            toolsVersion: '17.0.0',
            minToolsVersion: '16.0.0-aa',
            clusterUri: '/clusters/bar',
            toolsAutoUpdate: false,
          });
          break;
        case 'Unreachable':
          unreachableClusters.push({
            clusterUri: '/clusters/bar',
            errorMessage:
              'transport: Error while dialing: failed to dial: dial tcp\n' +
              '192.168.100.39:3080: connect: connection refused',
          });
      }
      return { reachableClusters, unreachableClusters };
    },
  });

  const updateInfo = makeUpdateInfo(
    storyProps.nonTeleportCdn,
    status.enabled ? status.version : '',
    storyProps.updateKind === 'Upgrade' ? 'upgrade' : 'downgrade'
  );

  switch (storyProps.step) {
    case 'Checking for update':
      return makeCheckingForUpdateEvent(status);
    case 'Update not available':
      return makeUpdateNotAvailableEvent(status);
    case 'Update available':
      if (status.enabled) {
        return makeUpdateAvailableEvent(updateInfo, status);
      }
      return;
    case 'Download progress':
      if (status.enabled) {
        return makeDownloadProgressEvent(updateInfo, status);
      }
      return;
    case 'Update downloaded':
      if (status.enabled) {
        return makeUpdateDownloadedEvent(updateInfo, status);
      }
      return;
    case 'Error':
      if (status.enabled) {
        return makeErrorEvent(updateInfo, status);
      }
      return;
  }
}

function WidgetAndDetails(storyProps: StoryProps) {
  const [state, setState] = useState<AppUpdateEvent>();
  useEffect(() => {
    resolveEvent(storyProps).then(setState);
  }, [storyProps]);

  if (!state) {
    return (
      <p>
        This step is only available when controls values allow resolving a
        client tools version.
        <br />
        Storybook does not support showing or hiding controls based on exact
        values of other controls.
      </p>
    );
  }

  // The two stacks with text should have the same height so that the widget view and the details
  // view both start at the same point when they're next to each other.
  const textHeight = '100px';

  return (
    <Flex rowGap={7} columnGap={10} flexWrap="wrap">
      <Stack gap={4} maxWidth="432px" alignItems="stretch">
        <Stack height={textHeight}>
          <H3>Widget View</H3>
          <P2>The component is rendered in the login form.</P2>
          <P2 mt={0}>
            If nothing is rendered, the widget is hidden in the state described
            by the story controls.
          </P2>
        </Stack>
        <WidgetView
          platform={storyProps.platform}
          updateEvent={state}
          clusterGetter={{
            findCluster: () => undefined,
          }}
          onMore={() => {}}
          onDownload={() => {}}
          onInstall={() => {}}
        />
      </Stack>
      <Stack gap={4} maxWidth="432px">
        <Stack height={textHeight}>
          <H3>Details View</H3>
          <P2>
            The details can be accessed through the widget if the widget is
            shown. Otherwise the user can access them through "Check for
            updates" in the additional actions menu.
          </P2>
        </Stack>
        <DetailsView
          platform={storyProps.platform}
          updateEvent={state}
          clusterGetter={{
            findCluster: () => undefined,
          }}
          changeManagingCluster={() => {}}
          onCheckForUpdates={() => {}}
          onDownload={() => {}}
          onCancelDownload={() => {}}
          onInstall={() => {}}
        />
      </Stack>
    </Flex>
  );
}

export const EnabledWithEnvVar: StoryObj<StoryProps> = {
  args: {
    envVar: 'Set to version - v15',
  },
};

export const EnabledWithSingleCluster: StoryObj<StoryProps> = {
  args: {
    clusterFoo: 'Enabled client updates - v18 cluster',
  },
};

export const EnabledWithHighestCompatible: StoryObj<StoryProps> = {
  args: {
    clusterFoo: 'Enabled client updates - v17 cluster',
    clusterBar: 'Enabled client updates - v16 cluster',
  },
};

export const EnabledWithManagingCluster: StoryObj<StoryProps> = {
  args: {
    clusterFoo: 'Enabled client updates - v17 cluster',
    clusterBar: 'Enabled client updates - v16 cluster',
    clusterBarSetToManageUpdates: true,
  },
};

export const EnabledWithManagingClusterAndSomeUnreachable: StoryObj<StoryProps> =
  {
    args: {
      clusterFoo: 'Unreachable',
      clusterBar: 'Enabled client updates - v16 cluster',
      clusterBarSetToManageUpdates: true,
    },
  };

export const DisabledBecauseSingleClusterUnreachable: StoryObj<StoryProps> = {
  args: {
    clusterFoo: 'Unreachable',
    clusterBar: 'Does not exist',
    step: 'Update not available',
  },
};
export const DisabledBecauseSingleClusterHasNoAutoupdates: StoryObj<StoryProps> =
  {
    args: {
      clusterFoo: 'Disabled client updates - v18 cluster',
      clusterBar: 'Does not exist',
      step: 'Update not available',
    },
  };

export const DisabledBecauseManagingClusterIsUnreachable: StoryObj<StoryProps> =
  {
    args: {
      clusterFoo: 'Enabled client updates - v17 cluster',
      clusterBar: 'Unreachable',
      clusterBarSetToManageUpdates: true,
      step: 'Update not available',
    },
  };

export const DisabledBecauseManagingClusterHasNoAutoupdates: StoryObj<StoryProps> =
  {
    args: {
      clusterFoo: 'Enabled client updates - v17 cluster',
      clusterBar: 'Disabled client updates - v17 cluster',
      clusterBarSetToManageUpdates: true,
      step: 'Update not available',
    },
  };

export const DisableBecauseNoClusterManagingUpdates: StoryObj<StoryProps> = {
  args: {
    clusterFoo: 'Disabled client updates - v18 cluster',
    clusterBar: 'Disabled client updates - v17 cluster',
    step: 'Update not available',
  },
};

export const DisabledBecauseClustersRequireIncompatibleVersions: StoryObj<StoryProps> =
  {
    args: {
      clusterFoo: 'Enabled client updates - v18 cluster',
      clusterBar: 'Enabled client updates - v16 cluster',
      step: 'Update not available',
    },
  };
