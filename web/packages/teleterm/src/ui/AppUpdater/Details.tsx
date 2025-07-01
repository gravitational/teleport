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

import { UpdateInfo } from 'electron-updater';
import { ReactNode, useState } from 'react';
import { JSX } from 'react/jsx-runtime';
import styled from 'styled-components';

import {
  Alert,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Indicator,
  Link,
  P3,
  Stack,
  Text,
} from 'design';
import { AlertKind } from 'design/Alert/Alert';
import { CheckboxInput } from 'design/Checkbox';
import { Checks, Info } from 'design/Icon';
import { RadioGroup } from 'design/RadioGroup';
import { P } from 'design/Text/Text';

import {
  AppUpdateEvent,
  AvailableVersion,
  CombinedEvent,
  ResolvedUpdateSource,
  UnresolvedUpdate,
  UpdateSource,
} from 'teleterm/services/appUpdater';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { formatMB, iconMac } from 'teleterm/ui/AppUpdater/common';
import { RootClusterUri } from 'teleterm/ui/uri';

export function Details({
  changeUpdatesSource,
  updateEvent,
  onCheckForUpdates,
  onInstall,
}: {
  updateEvent: AppUpdateEvent;
  onCheckForUpdates(): void;
  onInstall(): void;
  changeUpdatesSource: (
    source:
      | { kind: 'auto' }
      | { kind: 'cluster-override'; clusterUri: RootClusterUri }
  ) => void;
}) {
  return (
    <Stack gap={3} width="100%">
      {updateEvent.updateSource && (
        <ManagedUpdate
          updateSource={updateEvent.updateSource}
          isCheckingForUpdates={updateEvent.kind === 'checking-for-update'}
          changeUpdatesSource={changeUpdatesSource}
        />
      )}
      <Content
        updateEvent={updateEvent}
        onCheckForAppUpdates={onCheckForUpdates}
        onInstall={onInstall}
      />
    </Stack>
  );
}

function ManagedUpdate(props: {
  updateSource: UpdateSource;
  isCheckingForUpdates: boolean;
  changeUpdatesSource: (
    source:
      | { kind: 'auto' }
      | { kind: 'cluster-override'; clusterUri: RootClusterUri }
  ) => void;
}) {
  const appContext = useAppContext();
  const getClusterName = (clusterUri: RootClusterUri) => {
    const cluster = appContext.clustersService.findCluster(clusterUri);
    if (!cluster) {
      return clusterUri;
    }
    return cluster.name;
  };

  const { updateSource } = props;

  let alert: JSX.Element;
  if (updateSource.resolved === false) {
    const content = getUnresolvedUpdateContent(updateSource);
    alert = (
      <Alert
        width="100%"
        mb={0}
        kind="danger"
        details={content.description}
        secondaryAction={
          content.link ? { content: 'Docs', href: content.link } : undefined
        }
      >
        {content.title}
      </Alert>
    );
  } else {
    const content = getResolvedUpdateContent(updateSource, getClusterName);
    alert = (
      <Alert
        width="100%"
        mb={0}
        kind="neutral"
        icon={Info}
        details={content.description}
        secondaryAction={
          content.link ? { content: 'Docs', href: content.link } : undefined
        }
      >
        {content.title}
      </Alert>
    );
  }
  const isAutoManaged =
    updateSource.resolved && updateSource.source.kind === 'most-compatible';
  const isMostCompatibleCheckboxDisabled =
    props.isCheckingForUpdates ||
    (updateSource.resolved === false &&
      updateSource.reason === 'conflicting-cluster-versions');

  const availableVersions = getAvailableVersions(updateSource);
  const [localIsAutoManaged, setLocalIsAutoManaged] = useState(isAutoManaged);
  return (
    <>
      {alert}
      {availableVersions.length > 1 && (
        <Stack>
          <Text>
            Multiple clusters are configured to manage updates. By default, the
            most compatible version is installed, but you can choose which
            cluster controls the version.
          </Text>
          <label
            css={`
              gap: 4px;
              display: flex;
            `}
          >
            <CheckboxInput
              checked={localIsAutoManaged}
              disabled={isMostCompatibleCheckboxDisabled}
              onChange={e => {
                setLocalIsAutoManaged(e.target.checked);
                if (e.target.checked) {
                  props.changeUpdatesSource({ kind: 'auto' });
                }
              }}
            />
            Use the most compatible version from your clusters (recommended)
          </label>
          <Text>Or select cluster to manage updates:</Text>
          <RadioGroup
            gap={0}
            size="small"
            value={
              updateSource.resolved &&
              updateSource.source.kind === 'cluster-override' &&
              updateSource.source.clusterUri
            }
            onChange={value => {
              setLocalIsAutoManaged(false);
              props.changeUpdatesSource({
                kind: 'cluster-override',
                clusterUri: value,
              });
            }}
            options={availableVersions.map(v => ({
              disabled: localIsAutoManaged || props.isCheckingForUpdates,
              label: `${getClusterName(v.clusterUri)} (${v.version})`,
              value: v.clusterUri,
            }))}
            name={'availableVersions'}
          />
          <hr
            css={`
              width: 100%;
            `}
          />
        </Stack>
      )}
    </>
  );
}

function getAvailableVersions(updateSource: UpdateSource): AvailableVersion[] {
  if (
    updateSource.resolved === false &&
    updateSource.reason === 'conflicting-cluster-versions'
  ) {
    return updateSource.availableVersions;
  }

  if (updateSource.resolved === true) {
    switch (updateSource.source.kind) {
      case 'most-compatible':
        return updateSource.source.availableVersions;
      case 'cluster-override':
        return updateSource.source.availableVersions;
      default:
        return [];
    }
  }
  return [];
}

function getResolvedUpdateContent(
  updateSource: ResolvedUpdateSource,
  getClusterName: (clusterUri: RootClusterUri) => string
): {
  title: string;
  description: string;
  link?: string;
} {
  switch (updateSource.source.kind) {
    case 'env-var':
      return {
        title: 'Updates are managed',
        description: `The app was was configured to use version ${updateSource.version}.`,
      };
    case 'cluster-override':
      return {
        title: 'Updates are managed by the cluster',
        description: `The "${getClusterName(updateSource.source.clusterUri)}" cluster administrator requires client tools to use version ${updateSource.version}`,
      };
    case 'most-compatible':
      if (updateSource.source.clusterUris.length === 1) {
        return {
          title: 'Updates are managed by the cluster',
          description: `The "${getClusterName(updateSource.source.clusterUris.at(0))}" cluster administrator requires client tools to use version ${updateSource.version}`,
        };
      }
      return {
        title: 'Updates are managed by the clusters',
        description: `The ${updateSource.source.clusterUris.map(c => `"${getClusterName(c)}"`).join(', ')} cluster administrators requires client tools to use version ${updateSource.version}`,
      };
  }
}

function getUnresolvedUpdateContent(updateSource: UnresolvedUpdate): {
  title: string;
  description: ReactNode;
  link?: string;
} {
  switch (updateSource.reason) {
    case 'disabled-by-env-var':
      return {
        title: 'Client updates are disabled',
        description: `Your local configuration disabled autoupdates.`,
      };
    case 'no-cdn-set':
      return {
        title: 'Client updates are disabled',
        description: (
          <>
            Client tools updates are disabled as they are licensed under AGPL.
            To use Community Edition builds or custom binaries, set the{' '}
            <code>TELEPORT_CDN_BASE_URL</code> environment variable.
          </>
        ),
      };
    case 'no-cluster-with-autoupdate':
      return {
        title: 'Client updates are not configured',
        description: (
          <>
            <Link href="https://goteleport.com/docs/upgrading/automatic-updates">
              Enroll
            </Link>{' '}
            in automatic updates to keep Teleport Connect automatically updated.
          </>
        ),
      };
    case 'conflicting-cluster-versions':
      return {
        title: 'Client updates are disabled',
        description: 'You clusters do not allow to find a compatible version',
      };
  }
}

function Content({
  updateEvent,
  onCheckForAppUpdates,
  onInstall,
}: {
  updateEvent: AppUpdateEvent;
  onCheckForAppUpdates(): void;
  onInstall(): void;
}) {
  switch (updateEvent.kind) {
    case 'checking-for-update':
      return (
        <Stack gap={3} width="100%">
          <Flex gap={1} alignItems="center">
            <Indicator mb={-1} size="medium" delay="none" />
            <P>Checking for updates…</P>
          </Flex>
          <ButtonPrimary
            block
            disabled
            onClick={() => {
              onCheckForAppUpdates();
            }}
          >
            Check For Updates
          </ButtonPrimary>
        </Stack>
      );
    case 'update-available':
      return (
        <Stack gap={3} width="100%">
          <Stack>
            <Text>
              A new version is available. View release notes on{' '}
              <Link href="https://github.com/gravitational/teleport/releases/tag/v16.5.10">
                GitHub
              </Link>
              .
            </Text>
            <AvailableUpdate update={updateEvent.update} />
          </Stack>
          <ButtonSecondary disabled block>
            Starting Download…
          </ButtonSecondary>
        </Stack>
      );
    case 'update-not-available':
      return (
        <Stack gap={3} width="100%">
          {updateEvent.updateSource.resolved && (
            <Flex gap={1}>
              <Checks color="success.main" size="medium" />
              <P>Teleport Connect is up to date.</P>
            </Flex>
          )}
          <ButtonSecondary
            block
            disabled={
              updateEvent.updateSource.resolved === false &&
              (updateEvent.updateSource.reason === 'disabled-by-env-var' ||
                updateEvent.updateSource.reason === 'no-cdn-set')
            }
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
          <Stack width="100%" gap={3}>
            {updateEvent.update && (
              <Stack>
                <Text>
                  A new version is available. View release notes on{' '}
                  <Link>GitHub</Link>.
                </Text>
                <AvailableUpdate update={updateEvent.update} />
              </Stack>
            )}
            <Alert width="100%" mb={0} details={updateEvent.error.message}>
              An error occurred
            </Alert>
          </Stack>
          <ButtonSecondary
            block
            onClick={() => {
              onCheckForAppUpdates();
            }}
          >
            Try Again
          </ButtonSecondary>
        </Stack>
      );
    case 'download-progress':
      return (
        <Stack gap={3} width="100%">
          <Stack width="100%">
            <Text>
              A new version is available. View release notes on{' '}
              <Link>GitHub</Link>.
            </Text>
            <AvailableUpdate update={updateEvent.update} />
            <label
              css={`
                width: 100%;
              `}
            >
              {/*Downloading update...*/}
              <Progress value={updateEvent.progress.percent} max="100" />
              <P3 color="text.slightlyMuted">{`Downloaded ${formatMB(updateEvent.progress.transferred)} of ${formatMB(updateEvent.progress.total)}`}</P3>
            </label>
          </Stack>
          <ButtonSecondary block>Cancel</ButtonSecondary>
        </Stack>
      );
    case 'update-downloaded':
      return (
        <Stack gap={3} width="100%">
          <Stack width="100%">
            <Text>
              A new version is available. View release notes on{' '}
              <Link>GitHub</Link>.
            </Text>
            <AvailableUpdate update={updateEvent.update} />
            <label
              css={`
                width: 100%;
              `}
            >
              {/*Downloading update...*/}
              <Progress value="100" max="100" />
              <P3 color="text.slightlyMuted">Update downloaded</P3>
            </label>
          </Stack>
          <ButtonPrimary block onClick={() => onInstall()}>
            Restart
          </ButtonPrimary>
        </Stack>
      );
  }
}

const Progress = styled.progress`
  width: 100%;
`;

function AvailableUpdate(props: { update: UpdateInfo }) {
  return (
    <Flex gap={1} alignItems="center">
      <img height="50px" src={iconMac} />
      <Stack gap={0}>
        <Text bold>Teleport Connect {props.update.version}</Text>
        <P3 color="text.slightlyMuted">{formatMB(149652576)}</P3>
      </Stack>
    </Flex>
  );
}
