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

import { useId, useState } from 'react';

import {
  Alert,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Indicator,
  Link,
  P2,
  P3,
  Stack,
  Text,
} from 'design';
import { Checks, Info } from 'design/Icon';

import { Platform } from 'teleterm/mainProcess/types';
import { AppUpdateEvent, UpdateInfo } from 'teleterm/services/appUpdater';
import { UnsupportedVersionError } from 'teleterm/services/appUpdater/errors';
import { RootClusterUri } from 'teleterm/ui/uri';

import { AutoUpdatesManagement } from './AutoUpdatesManagement';
import {
  ClusterGetter,
  formatMB,
  iconMac,
  iconWinLinux,
  isTeleportDownloadHost,
} from './common';

/**
 * Detailed updates view.
 * The details can be accessed through the widget if the widget is
 * shown.
 * Otherwise, the user can access them through "Check for
 * updates" in the additional actions menu.
 */
export function DetailsView({
  changeManagingCluster,
  clusterGetter,
  updateEvent,
  platform,
  onCheckForUpdates,
  onDownload,
  onCancelDownload,
  onInstall,
}: {
  updateEvent: AppUpdateEvent;
  platform: Platform;
  clusterGetter: ClusterGetter;
  onCheckForUpdates(): void;
  onInstall(): void;
  onDownload(): void;
  onCancelDownload(): void;
  changeManagingCluster(clusterUri: RootClusterUri | undefined): void;
}) {
  return (
    <Stack gap={3} width="100%">
      {updateEvent.autoUpdatesStatus && (
        <AutoUpdatesManagement
          clusterGetter={clusterGetter}
          status={updateEvent.autoUpdatesStatus}
          updateEventKind={updateEvent.kind}
          onCheckForUpdates={onCheckForUpdates}
          changeManagingCluster={changeManagingCluster}
        />
      )}
      <UpdaterState
        event={updateEvent}
        platform={platform}
        onCheckForAppUpdates={onCheckForUpdates}
        onDownload={onDownload}
        onCancelDownload={onCancelDownload}
        onInstall={onInstall}
        key={JSON.stringify(updateEvent)}
      />
    </Stack>
  );
}

function UpdaterState({
  event,
  platform,
  onCheckForAppUpdates,
  onDownload,
  onCancelDownload,
  onInstall,
}: {
  event: AppUpdateEvent;
  platform: Platform;
  onCheckForAppUpdates(): void;
  onDownload(): void;
  onCancelDownload(): void;
  onInstall(): void;
}) {
  const [downloadStarted, setDownloadStarted] = useState(false);
  switch (event.kind) {
    case 'checking-for-update':
      return (
        <Stack gap={3} width="100%">
          <Flex gap={1} alignItems="center">
            <Indicator mb={-1} size="medium" delay="none" />
            <P2>Checking for updates…</P2>
          </Flex>
          <ButtonPrimary block disabled onClick={() => onCheckForAppUpdates()}>
            Check For Updates
          </ButtonPrimary>
        </Stack>
      );
    case 'update-available':
      return (
        <Stack gap={3} width="100%">
          <AvailableUpdate update={event.update} platform={platform} />
          {event.autoDownload || downloadStarted ? (
            <ButtonSecondary disabled block>
              Starting Download…
            </ButtonSecondary>
          ) : (
            <ButtonSecondary
              block
              onClick={() => {
                setDownloadStarted(true);
                onDownload();
              }}
            >
              Download
            </ButtonSecondary>
          )}
        </Stack>
      );
    case 'update-not-available':
      return (
        <Stack gap={3} width="100%">
          {event.autoUpdatesStatus.enabled && (
            <Flex gap={1}>
              <Checks color="success.main" size="medium" />
              <P2>Teleport Connect is up to date.</P2>
            </Flex>
          )}
          <ButtonSecondary
            block
            disabled={!event.autoUpdatesStatus.enabled}
            onClick={() => {
              onCheckForAppUpdates();
            }}
          >
            Check For Updates
          </ButtonSecondary>
        </Stack>
      );
    case 'error':
      return (
        <Stack gap={3} width="100%">
          {event.update && (
            <AvailableUpdate update={event.update} platform={platform} />
          )}
          <Alert mb={1} details={event.error.message} width="100%">
            {event.update
              ? 'Update failed'
              : event.error.name === UnsupportedVersionError.name
                ? 'Incompatible managed update version'
                : 'Unable to check for app updates'}
          </Alert>
          <ButtonSecondary block onClick={onCheckForAppUpdates}>
            Try Again
          </ButtonSecondary>
        </Stack>
      );
    case 'download-progress':
      return (
        <Stack gap={3} width="100%">
          <Stack width="100%">
            <AvailableUpdate update={event.update} platform={platform} />
            <Progress
              progressPercent={event.progress.percent}
              label={`Downloaded ${formatMB(event.progress.transferred)} of ${formatMB(event.progress.total)}`}
            />
          </Stack>
          <ButtonSecondary block onClick={onCancelDownload}>
            Cancel
          </ButtonSecondary>
        </Stack>
      );
    case 'update-downloaded':
      const label =
        platform === 'darwin'
          ? 'Ready to install'
          : 'Ready to install. Admin permissions may be required.';
      return (
        <Stack gap={3} width="100%">
          <Stack width="100%">
            <AvailableUpdate update={event.update} platform={platform} />
            <Progress progressPercent={100} label={label} />
          </Stack>
          <ButtonPrimary block onClick={onInstall}>
            Restart
          </ButtonPrimary>
        </Stack>
      );
  }
}

function AvailableUpdate(props: { update: UpdateInfo; platform: Platform }) {
  const downloadHost = new URL(props.update.files.at(0).url).host;
  const isOfficialServer = isTeleportDownloadHost(downloadHost);

  return (
    <Stack>
      <Text>A new version is available.</Text>
      <Flex gap={1} alignItems="center">
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
          <Text bold>Teleport Connect {props.update.version}</Text>
          <P3 color="text.slightlyMuted">
            <Link
              target="_blank"
              href={`https://github.com/gravitational/teleport/releases/tag/v${props.update.version}`}
              css={`
                display: inline-flex;
                gap: 3px;
              `}
            >
              Release notes
            </Link>
          </P3>
        </Stack>
      </Flex>
      <P3 m={0} color="text.slightlyMuted">
        {!isOfficialServer && (
          <Flex gap={1} mt={1}>
            <Info size="small" />
            Using {downloadHost} as the update server.
          </Flex>
        )}
      </P3>
    </Stack>
  );
}

function Progress(props: { progressPercent: number; label: string }) {
  const labelId = useId();
  return (
    <Stack gap={0} width="100%">
      <progress
        style={{ width: '100%' }}
        aria-labelledby={labelId}
        value={props.progressPercent}
        max="100"
      />
      <P3 id={labelId} color="text.slightlyMuted">
        {props.label}
      </P3>
    </Stack>
  );
}
