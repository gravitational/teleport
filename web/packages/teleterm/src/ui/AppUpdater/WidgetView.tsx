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

import { ComponentType } from 'react';

import {
  ButtonBorder,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  P3,
  Stack,
  Text,
} from 'design';
import { Alert } from 'design/Alert';
import { Info } from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import { SpaceProps } from 'design/system';
import { UnreachableCluster } from 'gen-proto-ts/teleport/lib/teleterm/auto_update/v1/auto_update_service_pb';

import { Platform } from 'teleterm/mainProcess/types';
import {
  AppUpdateEvent,
  AutoUpdatesStatus,
} from 'teleterm/services/appUpdater';
import { UnsupportedVersionError } from 'teleterm/services/appUpdater/errors';
import { RootClusterUri } from 'teleterm/ui/uri';

import {
  ClusterGetter,
  clusterNameGetter,
  formatMB,
  getDownloadHost,
  iconMac,
  iconWinLinux,
  isTeleportDownloadHost,
  makeUnreachableClusterText,
} from './common';

/**
 * App updates widget.
 * The component is rendered in the login form.
 *
 * Hidden for `update-not-available` and `checking-for-update` events,
 * unless there's an issue that prevents autoupdates from working.
 */
export function WidgetView({
  clusterGetter,
  onDownload,
  onInstall,
  onMore,
  platform,
  updateEvent,
  ...rest
}: {
  updateEvent: AppUpdateEvent;
  platform: Platform;
  clusterGetter: ClusterGetter;
  onMore(): void;
  onDownload(): void;
  onInstall(): void;
} & SpaceProps) {
  const getClusterName = clusterNameGetter(clusterGetter);
  const { autoUpdatesStatus } = updateEvent;

  const issueRequiringAttention =
    autoUpdatesStatus &&
    findAutoUpdatesIssuesRequiringAttention(autoUpdatesStatus, getClusterName);

  if (issueRequiringAttention) {
    return (
      <Alert
        kind="danger"
        mb={0}
        {...rest}
        details={
          <Stack gap={2}>
            {issueRequiringAttention}
            {/*TODO(gzdunek): Allow Alert to show buttons at the bottom. */}
            <ButtonBorder onClick={onMore}>Resolve</ButtonBorder>
          </Stack>
        }
      >
        App updates are disabled
      </Alert>
    );
  }

  // If an error occurred when there was no update info, return early.
  if (updateEvent.kind === 'error' && !updateEvent.update) {
    return (
      <Alert
        kind="danger"
        mb={0}
        {...rest}
        details={
          <Stack gap={2}>
            {updateEvent.error.message}
            {/*TODO(gzdunek): Allow Alert to show buttons at the bottom. */}
            <ButtonBorder onClick={onMore}>More</ButtonBorder>
          </Stack>
        }
      >
        {updateEvent.error.name === UnsupportedVersionError.name
          ? 'Incompatible managed update version'
          : 'Unable to check for app updates'}
      </Alert>
    );
  }

  if (
    updateEvent.kind === 'checking-for-update' ||
    updateEvent.kind === 'update-not-available'
  ) {
    return;
  }

  const { description, button } = makeUpdaterContent({
    updateEvent,
    onDownload,
    onInstall,
  });

  const unreachableClusters =
    // It's important only when the highest compatible version was found.
    updateEvent.autoUpdatesStatus.source === 'highest-compatible'
      ? updateEvent.autoUpdatesStatus.options.unreachableClusters
      : [];
  const downloadBaseUrl = getDownloadHost(updateEvent);

  return (
    <AvailableUpdate
      version={updateEvent.update.version}
      platform={platform}
      description={description}
      unreachableClusters={unreachableClusters}
      downloadHost={downloadBaseUrl}
      onMore={onMore}
      getClusterName={getClusterName}
      primaryButton={
        button ? { name: button.name, onClick: button.action } : undefined
      }
      {...rest}
    />
  );
}

function AvailableUpdate({
  description,
  downloadHost,
  onMore,
  platform,
  primaryButton,
  unreachableClusters,
  version,
  ...rest
}: {
  version: string;
  description: string;
  unreachableClusters: UnreachableCluster[];
  downloadHost: string;
  platform: Platform;
  onMore(): void;
  getClusterName(clusterUri: RootClusterUri): string;
  primaryButton?: {
    name: string;
    onClick(): void;
  };
} & SpaceProps) {
  const hasUnreachableClusters = !!unreachableClusters.length;
  const isNonTeleportServer =
    downloadHost && !isTeleportDownloadHost(downloadHost);

  return (
    // Mimics a neutral alert.
    <Stack
      justifyContent="space-between"
      gap={1}
      css={`
        border: 1px solid ${props => props.theme.colors.text.disabled};
        background: ${props => props.theme.colors.interactive.tonal.neutral[0]};
      `}
      borderRadius={3}
      px={3}
      py="12px"
      {...rest}
    >
      <Flex width="100%" alignItems="center" justifyContent="space-between">
        <Flex gap={1} alignItems="center" width="100%">
          {platform === 'darwin' ? (
            <img alt="App icon" height="50px" src={iconMac} />
          ) : (
            <img
              alt="App icon"
              height="43px"
              style={{ marginRight: '4px' }}
              src={iconWinLinux}
            />
          )}
          <Stack gap={0}>
            <Text bold>Teleport Connect {version}</Text>
            <P3>{description}</P3>
          </Stack>
        </Flex>
        <Flex gap={2}>
          {primaryButton && (
            <ButtonPrimary size="small" onClick={primaryButton.onClick}>
              {primaryButton.name}
            </ButtonPrimary>
          )}
          <ButtonSecondary size="small" onClick={onMore}>
            More
          </ButtonSecondary>
        </Flex>
      </Flex>
      {(hasUnreachableClusters || isNonTeleportServer) && (
        <Stack ml={1}>
          {hasUnreachableClusters && (
            <IconAndText
              Icon={Info}
              text="Unable to retrieve accepted client versions from some clusters."
            />
          )}
          {isNonTeleportServer && (
            <IconAndText
              Icon={Info}
              text={`Using ${downloadHost} as the update server.`}
            />
          )}
        </Stack>
      )}
    </Stack>
  );
}

function IconAndText(props: { Icon: ComponentType<IconProps>; text: string }) {
  return (
    <Flex gap={1} color="text.slightlyMuted" alignItems="start">
      <props.Icon size="small" />
      <P3
        css={`
          line-height: normal;
        `}
      >
        {props.text}
      </P3>
    </Flex>
  );
}

function makeUpdaterContent({
  updateEvent,
  onInstall,
  onDownload,
}: {
  updateEvent: AppUpdateEvent;
  onDownload(): void;
  onInstall(): void;
}): {
  description: string;
  button?: {
    name: string;
    action(): void;
  };
} {
  switch (updateEvent.kind) {
    case 'download-progress':
      return {
        description: `Downloaded ${formatMB(updateEvent.progress.transferred)} of ${formatMB(updateEvent.progress.total)}`,
      };
    case 'update-available':
      const { updateKind } = updateEvent.update;
      if (updateEvent.autoDownload) {
        return {
          description:
            updateKind === 'upgrade'
              ? 'Update available. Starting download…'
              : 'Downloading required version…',
        };
      }

      return {
        description:
          updateKind === 'upgrade'
            ? 'Update available'
            : 'Downgrade to required version',
        button: {
          name: 'Download',
          action: onDownload,
        },
      };
    case 'update-downloaded':
      return {
        description: 'Ready to install',
        button: {
          name: 'Restart',
          action: onInstall,
        },
      };
    case 'error':
      return {
        description: 'Update failed',
      };
  }
}

/** Returns issues that need to be resolved to make autoupdates work. */
function findAutoUpdatesIssuesRequiringAttention(
  status: AutoUpdatesStatus,
  getClusterName: (clusterUri: RootClusterUri) => string
): string | undefined {
  if (status.enabled === false && status.reason === 'no-compatible-version') {
    return 'Your clusters require incompatible client versions. Choose one to enable app updates.';
  }

  if (
    status.enabled === false &&
    status.reason === 'managing-cluster-unable-to-manage'
  ) {
    return `The cluster ${getClusterName(status.options.managingClusterUri)} was chosen to manage updates but is not able to provide them.`;
  }

  if (
    status.enabled === false &&
    status.reason === 'no-cluster-with-auto-update' &&
    status.options.unreachableClusters.length
  ) {
    return makeUnreachableClusterText(
      status.options.unreachableClusters,
      getClusterName
    );
  }
}
