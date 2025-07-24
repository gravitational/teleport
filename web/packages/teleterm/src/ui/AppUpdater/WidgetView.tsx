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

import { ButtonPrimary, ButtonSecondary, Flex, P3, Stack, Text } from 'design';
import { Alert } from 'design/Alert';
import { Info, Warning } from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import { UnreachableCluster } from 'gen-proto-ts/teleport/lib/teleterm/auto_update/v1/auto_update_service_pb';
import { getErrorMessage } from 'shared/utils/error';

import { Platform } from 'teleterm/mainProcess/types';
import { AppUpdateEvent } from 'teleterm/services/appUpdater';
import { RootClusterUri } from 'teleterm/ui/uri';

import {
  ClusterGetter,
  clusterNameGetter,
  findUnreachableClusters,
  formatMB,
  getDownloadHost,
  iconMac,
  iconWinLinux,
  isTeleportDownloadHost,
} from './common';

/**
 * App updates widget.
 * The widget is hidden for `update-not-available` and `checking-for-update`
 * events.
 */
export function WidgetView(props: {
  updateEvent: AppUpdateEvent;
  platform: Platform;
  clusterGetter: ClusterGetter;
  onMore(): void;
  onDownload(): void;
  onInstall(): void;
}) {
  const getClusterName = clusterNameGetter(props.clusterGetter);
  const { updateEvent } = props;

  if (
    updateEvent.autoUpdatesStatus.enabled === false &&
    updateEvent.autoUpdatesStatus.reason === 'no-compatible-version'
  ) {
    return (
      <Alert
        kind="danger"
        width="100%"
        details="Your clusters require incompatible client versions. Choose one to enable app updates."
        secondaryAction={{
          content: 'Resolve',
          onClick: props.onMore,
        }}
      >
        App updates are disabled
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
    onDownload: props.onDownload,
    onInstall: props.onInstall,
  });

  const unreachableClusters =
    // It's important only when the most compatible version was found.
    updateEvent.autoUpdatesStatus.source.kind === 'most-compatible'
      ? findUnreachableClusters(updateEvent.autoUpdatesStatus)
      : [];
  const downloadBaseUrl = getDownloadHost(updateEvent);
  const skippedManagingClusterUri =
    updateEvent.autoUpdatesStatus.source.kind === 'most-compatible'
      ? updateEvent.autoUpdatesStatus.source.skippedManagingClusterUri
      : '';

  return (
    <AvailableUpdate
      version={updateEvent.update.version}
      platform={props.platform}
      description={description}
      unreachableClusters={unreachableClusters}
      skippedManagingClusterUri={skippedManagingClusterUri}
      downloadHost={downloadBaseUrl}
      onMore={props.onMore}
      getClusterName={getClusterName}
      primaryButton={
        button ? { name: button.name, onClick: button.action } : undefined
      }
    />
  );
}

function AvailableUpdate(props: {
  version: string;
  description: string | { Icon: ComponentType<IconProps>; text: string };
  unreachableClusters: UnreachableCluster[];
  skippedManagingClusterUri: string;
  downloadHost: string;
  platform: Platform;
  onMore(): void;
  getClusterName(clusterUri: RootClusterUri): string;
  primaryButton?: {
    name: string;
    onClick(): void;
  };
}) {
  const hasUnreachableClusters = !!props.unreachableClusters.length;
  const isNonTeleportServer =
    props.downloadHost && !isTeleportDownloadHost(props.downloadHost);

  return (
    // Mimics a neutral alert.
    <Stack
      justifyContent="space-between"
      gap={1}
      width="100%"
      css={`
        border: 1px solid ${props => props.theme.colors.text.disabled};
        background: ${props => props.theme.colors.interactive.tonal.neutral[0]};
      `}
      borderRadius={3}
      px={3}
      py="12px"
    >
      <Flex width="100%" alignItems="center" justifyContent="space-between">
        <Flex gap={1} alignItems="center" width="100%">
          {props.platform === 'darwin' ? (
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
            <Text bold>Teleport Connect {props.version}</Text>
            {typeof props.description === 'object' ? (
              <Flex gap={1}>
                <props.description.Icon size="small" />
                <P3>{props.description.text}</P3>
              </Flex>
            ) : (
              <P3>{props.description}</P3>
            )}
          </Stack>
        </Flex>
        <Flex gap={2}>
          {props.primaryButton && (
            <ButtonPrimary size="small" onClick={props.primaryButton.onClick}>
              {props.primaryButton.name}
            </ButtonPrimary>
          )}
          <ButtonSecondary size="small" onClick={props.onMore}>
            More
          </ButtonSecondary>
        </Flex>
      </Flex>
      {(hasUnreachableClusters ||
        props.skippedManagingClusterUri ||
        isNonTeleportServer) && (
        <Stack gap={0} ml={1}>
          {props.skippedManagingClusterUri && (
            <Flex gap={1} color="text.slightlyMuted">
              <Warning size="small" />
              <P3>
                Cluster {props.getClusterName(props.skippedManagingClusterUri)}{' '}
                is not managing updates.
              </P3>
            </Flex>
          )}
          {hasUnreachableClusters && (
            <Flex gap={1} color="text.slightlyMuted">
              <Warning size="small" />
              <P3>
                Unable to retrieve accepted client versions from some clusters.
              </P3>
            </Flex>
          )}
          {isNonTeleportServer && (
            <Flex gap={1} color="text.slightlyMuted">
              <Info size="small" />
              <P3>Using {props.downloadHost} as the update server.</P3>
            </Flex>
          )}
        </Stack>
      )}
    </Stack>
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
  description: string | { Icon: ComponentType<IconProps>; text: string };
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
      if (updateEvent.autoDownload) {
        return {
          description: 'Update available. Starting download…',
        };
      }
      return {
        description: 'Update available',
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
        description: {
          Icon: Warning,
          text: getErrorMessage(updateEvent.error),
        },
      };
  }
}
