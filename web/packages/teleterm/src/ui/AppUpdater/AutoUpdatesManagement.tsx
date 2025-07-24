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

import { ReactNode, useState } from 'react';

import { Alert } from 'design/Alert';
import { CheckboxInput } from 'design/Checkbox';
import { Stack } from 'design/Flex';
import { Cog } from 'design/Icon';
import Link from 'design/Link';
import { RadioGroup } from 'design/RadioGroup';
import Text from 'design/Text';
import { UnreachableCluster } from 'gen-proto-ts/teleport/lib/teleterm/auto_update/v1/auto_update_service_pb';

import {
  AppUpdateEvent,
  AutoUpdatesDisabled,
  AutoUpdatesEnabled,
  AutoUpdatesStatus,
  Cluster,
} from 'teleterm/services/appUpdater';
import { RootClusterUri } from 'teleterm/ui/uri';

import {
  ClusterGetter,
  clusterNameGetter,
  findUnreachableClusters,
} from './common';

export function AutoUpdatesManagement(props: {
  status: AutoUpdatesStatus;
  updateEventKind: AppUpdateEvent['kind'];
  clusterGetter: ClusterGetter;
  changeManagingCluster(clusterUri: RootClusterUri | undefined): void;
  onCheckForUpdates(): void;
}) {
  const { status } = props;

  const unreachableClusters = findUnreachableClusters(status);
  const getClusterName = clusterNameGetter(props.clusterGetter);
  const content =
    status.enabled === true
      ? makeContentForEnabledAutoUpdates(status, getClusterName)
      : makeContentForDisabledAutoUpdates(status);

  return (
    <>
      {unreachableClusters.length > 0 && (
        <Alert
          width="100%"
          mb={0}
          kind="warning"
          primaryAction={{
            content: 'Refresh',
            onClick: props.onCheckForUpdates,
          }}
          details={
            `Unable to retrieve accepted client versions from ${unreachableClusters.map(c => getClusterName(c.clusterUri)).join(', ')}. ` +
            `Compatibility with ${unreachableClusters.length === 1 ? 'this cluster' : 'these clusters'} will not be verified.`
          }
        >
          Unreachable clusters
        </Alert>
      )}
      <Alert
        width="100%"
        mb={0}
        icon={content.kind === 'neutral' ? Cog : undefined}
        kind={content.kind}
        details={content.description}
      >
        {'title' in content ? content.title : ''}
      </Alert>
      <ManagingClusterSelector
        autoUpdatesStatus={status}
        changeManagingCluster={props.changeManagingCluster}
        isCheckingForUpdates={props.updateEventKind === 'checking-for-update'}
        getClusterName={getClusterName}
        // Resets localIsAutoManaged checkbox.
        key={JSON.stringify(status)}
      />
    </>
  );
}

function ManagingClusterSelector({
  autoUpdatesStatus,
  isCheckingForUpdates,
  changeManagingCluster,
  getClusterName,
}: {
  autoUpdatesStatus: AutoUpdatesStatus;
  isCheckingForUpdates: boolean;
  changeManagingCluster(clusterUri: RootClusterUri | undefined): void;
  getClusterName(clusterUri: RootClusterUri): string;
}) {
  const isAutoManaged =
    autoUpdatesStatus.enabled &&
    autoUpdatesStatus.source.kind === 'most-compatible';
  // A local state allows us to unselect the checkbox without choosing any managing cluster.
  const [localIsAutoManaged, setLocalIsAutoManaged] = useState(isAutoManaged);

  const isMostCompatibleCheckboxDisabled =
    isCheckingForUpdates ||
    (autoUpdatesStatus.enabled === false &&
      autoUpdatesStatus.reason === 'no-compatible-version');

  const options = makeOptions(autoUpdatesStatus, getClusterName);

  return (
    <>
      {options.length > 1 && (
        <Stack width="100%">
          <label
            css={`
              gap: ${p => p.theme.space[1]}px;
              display: flex;
            `}
          >
            <CheckboxInput
              checked={localIsAutoManaged}
              disabled={isMostCompatibleCheckboxDisabled}
              onChange={e => {
                setLocalIsAutoManaged(e.target.checked);
                if (e.target.checked) {
                  changeManagingCluster(undefined);
                }
              }}
            />
            Use the most compatible version from your clusters
          </label>
          {!localIsAutoManaged && (
            <>
              <Text>Choose which cluster should manage updates:</Text>
              <RadioGroup
                gap={0}
                name="managingCluster"
                size="small"
                value={
                  autoUpdatesStatus.enabled &&
                  autoUpdatesStatus.source.kind === 'managing-cluster' &&
                  autoUpdatesStatus.source.clusterUri
                }
                onChange={clusterUri => {
                  setLocalIsAutoManaged(false);
                  changeManagingCluster(clusterUri);
                }}
                options={options}
              />
            </>
          )}
          <hr
            css={`
              border-color: ${props => props.theme.colors.spotBackground[2]};
              border-style: solid;
              width: 100%;
              margin-bottom: 0;
              margin-top: ${p => p.theme.space[2]}px;
            `}
          />
        </Stack>
      )}
    </>
  );
}

function makeOptions(
  status: AutoUpdatesStatus,
  getClusterName: (clusterUri: RootClusterUri) => string
) {
  let source:
    | {
        clusters: Cluster[];
        unreachableClusters: UnreachableCluster[];
      }
    | undefined;

  if (status.enabled === false) {
    if (status.reason === 'no-compatible-version') {
      source = status;
    }
  } else {
    if (
      status.source.kind === 'most-compatible' ||
      status.source.kind === 'managing-cluster'
    ) {
      source = status.source;
    }
  }

  if (!source) {
    return [];
  }

  const candidateClusters = source.clusters
    .filter(c => c.toolsAutoUpdate)
    .map(c => {
      const otherNames = c.otherCompatibleClusters.map(c => getClusterName(c));
      const compatibility =
        otherNames.length === 0
          ? 'only compatible with this cluster'
          : `also compatible with ${otherNames.join(', ')}`;

      return {
        label: getClusterName(c.clusterUri),
        helperText: `${c.toolsVersion} client, ${compatibility}.`,
        value: c.clusterUri,
      };
    });

  const nonCandidateClusters = source.clusters
    .filter(c => !c.toolsAutoUpdate)
    .map(c => {
      return {
        disabled: true,
        label: getClusterName(c.clusterUri),
        helperText: `${c.toolsVersion} client, automatic client tools updates disabled on this cluster.`,
        value: c.clusterUri,
      };
    });

  const unreachableClusters = source.unreachableClusters.map(cluster => ({
    disabled: true,
    label: getClusterName(cluster.clusterUri),
    helperText: `⚠︎ Unreachable cluster: ${cluster.errorMessage}`,
    value: cluster.clusterUri,
  }));

  return [
    ...candidateClusters,
    ...nonCandidateClusters,
    ...unreachableClusters,
  ];
}

function makeContentForEnabledAutoUpdates(
  status: AutoUpdatesEnabled,
  getClusterName: (clusterUri: RootClusterUri) => string
): {
  description: string;
  kind: 'neutral' | 'warning';
} {
  switch (status.source.kind) {
    case 'env-var':
      return {
        kind: 'neutral',
        description: `The app is set to stay on version ${status.version} by your device settings.`,
      };
    case 'managing-cluster':
      return {
        kind: 'neutral',
        description: `Updates are managed by the ${getClusterName(status.source.clusterUri)} cluster, which requires client version ${status.version}.`,
      };
    case 'most-compatible':
      const managingClusters = status.source.clusters
        .filter(c => c.toolsAutoUpdate && c.toolsVersion === status.version)
        .map(c => getClusterName(c.clusterUri));
      const { skippedManagingClusterUri } = status.source;
      if (skippedManagingClusterUri) {
        const skippedManagingClusterText = skippedManagingClusterUri
          ? `The chosen cluster ${getClusterName(skippedManagingClusterUri)} could not be used to manage updates`
          : '';
        return managingClusters.length === 1
          ? {
              kind: 'warning',
              description: `${skippedManagingClusterText}. Updates will be managed by the ${managingClusters.at(0)} cluster, which requires client version ${status.version}.`,
            }
          : {
              kind: 'warning',
              description: `${skippedManagingClusterText}. Updates will be managed by the ${managingClusters.map(c => c).join(', ')}, which require client version ${status.version}.`,
            };
      }

      return managingClusters.length === 1
        ? {
            kind: 'neutral',
            description: `Updates are managed by the ${managingClusters.at(0)} cluster, which requires client version ${status.version}.`,
          }
        : {
            kind: 'neutral',
            description: `Updates are managed by the ${managingClusters.map(c => c).join(', ')}, which require client version ${status.version}.`,
          };
  }
}

function makeContentForDisabledAutoUpdates(updateSource: AutoUpdatesDisabled): {
  title?: string;
  description?: ReactNode;
  kind: 'danger' | 'neutral';
} {
  switch (updateSource.reason) {
    case 'disabled-by-env-var':
      return {
        kind: 'neutral',
        description: 'Updates are disabled by your device settings.',
      };
    case 'no-cluster-with-auto-update':
      return {
        kind: 'neutral',
        title: 'App updates are not configured',
        description: (
          <>
            <Link href="https://goteleport.com/docs/upgrading/automatic-updates">
              Enroll
            </Link>{' '}
            in automatic updates to keep Teleport Connect updated.
          </>
        ),
      };
    case 'no-compatible-version':
      return {
        kind: 'danger',
        title: 'App updates are disabled',
        description:
          'Your clusters require incompatible client versions. To enable app updates, select which cluster should manage them.',
      };
  }
}
