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

import {
  AppUpdateEvent,
  AutoUpdatesDisabled,
  AutoUpdatesEnabled,
  AutoUpdatesStatus,
} from 'teleterm/services/appUpdater';
import { RootClusterUri } from 'teleterm/ui/uri';

import { ClusterGetter, clusterNameGetter } from './common';

const listFormatter = new Intl.ListFormat('en', {
  style: 'long',
  type: 'conjunction',
});

export function AutoUpdatesManagement(props: {
  status: AutoUpdatesStatus;
  updateEventKind: AppUpdateEvent['kind'];
  clusterGetter: ClusterGetter;
  changeManagingCluster(clusterUri: RootClusterUri | undefined): void;
  onCheckForUpdates(): void;
}) {
  const { status } = props;

  const { unreachableClusters } = status.options;
  const getClusterName = clusterNameGetter(props.clusterGetter);
  const content =
    status.enabled === true
      ? makeContentForEnabledAutoUpdates(status, getClusterName)
      : makeContentForDisabledAutoUpdates(status);

  const hasUnreachableClusters = unreachableClusters.length > 0;
  const unreachableDetailsText =
    `Unable to retrieve accepted client versions` +
    ` from the ${unreachableClusters.length === 1 ? 'cluster' : 'clusters'}` +
    ` ${listFormatter.format(unreachableClusters.map(c => getClusterName(c.clusterUri)))}.`;

  return (
    <>
      {hasUnreachableClusters && !content.showInlineUnreachableErrors && (
        <Alert
          width="100%"
          mb={0}
          kind="warning"
          primaryAction={{
            content: 'Refresh',
            onClick: props.onCheckForUpdates,
          }}
          details={unreachableDetailsText}
        >
          Unreachable clusters
        </Alert>
      )}
      {content && (
        <Alert
          width="100%"
          mb={0}
          icon={content.kind === 'neutral' ? Cog : undefined}
          kind={content.kind}
          details={
            hasUnreachableClusters && content.showInlineUnreachableErrors
              ? unreachableDetailsText
              : content.description
          }
          primaryAction={
            hasUnreachableClusters &&
            content.showInlineUnreachableErrors && {
              content: 'Refresh',
              onClick: props.onCheckForUpdates,
            }
          }
        >
          {'title' in content ? content.title : ''}
        </Alert>
      )}
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
    autoUpdatesStatus.source === 'highest-compatible';
  // A local state allows us to unselect the checkbox without choosing any managing cluster.
  const [localIsAutoManaged, setLocalIsAutoManaged] = useState(isAutoManaged);

  const isMostCompatibleCheckboxDisabled =
    isCheckingForUpdates || !autoUpdatesStatus.options.highestCompatibleVersion;
  const disabledClusterSelection = localIsAutoManaged;
  const options = makeOptions({
    status: autoUpdatesStatus,
    getClusterName: getClusterName,
    disabled: disabledClusterSelection,
  });

  return (
    <>
      {(options.length > 1 || autoUpdatesStatus.options.managingClusterUri) && (
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
            Use the highest compatible version from your clusters
          </label>
          <>
            <Text color={disabledClusterSelection ? 'disabled' : undefined}>
              Choose which cluster should manage updates:
            </Text>
            <RadioGroup
              gap={0}
              name="managingCluster"
              size="small"
              value={autoUpdatesStatus.options.managingClusterUri}
              onChange={clusterUri => {
                setLocalIsAutoManaged(false);
                changeManagingCluster(clusterUri);
              }}
              options={options}
            />
          </>
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

function makeOptions({
  status,
  getClusterName,
  disabled,
}: {
  status: AutoUpdatesStatus;
  getClusterName: (clusterUri: RootClusterUri) => string;
  disabled: boolean;
}) {
  const candidateClusters = status.options.clusters
    .filter(c => c.toolsAutoUpdate)
    .map(c => {
      const otherNames = c.otherCompatibleClusters.map(c => getClusterName(c));
      const compatibility =
        otherNames.length === 0
          ? 'only compatible with this cluster'
          : `also compatible with ${listFormatter.format(otherNames)}`;

      return {
        disabled,
        label: getClusterName(c.clusterUri),
        helperText: `${c.toolsVersion} client, ${compatibility}.`,
        value: c.clusterUri,
      };
    });

  const nonCandidateClusters = status.options.clusters
    .filter(c => !c.toolsAutoUpdate)
    .map(c => {
      return {
        disabled,
        label: getClusterName(c.clusterUri),
        helperText: (
          <>
            {c.toolsVersion} client.
            <br />
            ⚠︎ Cannot provide updates, automatic client tools updates are
            disabled on this cluster.
          </>
        ),
        value: c.clusterUri,
      };
    });

  const unreachableClusters = status.options.unreachableClusters.map(
    cluster => ({
      disabled,
      label: getClusterName(cluster.clusterUri),
      helperText: (
        <>
          ⚠︎ Cannot provide updates, cluster is unreachable.
          <br />
          {cluster.errorMessage}
        </>
      ),
      value: cluster.clusterUri,
    })
  );

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
  showInlineUnreachableErrors?: boolean;
} {
  switch (status.source) {
    case 'env-var':
      return {
        kind: 'neutral',
        description: `The app is set to stay on version ${status.version} by your device settings.`,
      };
    case 'managing-cluster':
      return {
        kind: 'neutral',
        description: `Updates are managed by the ${getClusterName(status.options.managingClusterUri)} cluster, which requires client version ${status.version}.`,
      };
    case 'highest-compatible':
      const managingClusters = status.options.clusters
        .filter(c => c.toolsAutoUpdate && c.toolsVersion === status.version)
        .map(c => getClusterName(c.clusterUri));
      return managingClusters.length === 1
        ? {
            kind: 'neutral',
            description: `Updates are managed by the cluster ${managingClusters.at(0)}, which requires client version ${status.version}.`,
          }
        : {
            kind: 'neutral',
            description: `Updates are managed by the clusters ${listFormatter.format(managingClusters.map(c => c))}, which require client version ${status.version}.`,
          };
  }
}

function makeContentForDisabledAutoUpdates(updateSource: AutoUpdatesDisabled): {
  title?: string;
  description?: ReactNode;
  kind: 'danger' | 'neutral';
  /** Replace description with "unreachable" error. */
  showInlineUnreachableErrors?: boolean;
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
        title: 'App updates are disabled',
        // If there are unreachable clusters, we don't want to show the enrollment info.
        showInlineUnreachableErrors: true,
        description: (
          <>
            The cluster{' '}
            <Link href="https://goteleport.com/docs/upgrading/automatic-updates">
              needs to enroll
            </Link>{' '}
            in automatic updates to keep Teleport Connect updated.
          </>
        ),
      };
    case 'managing-cluster-unable-to-manage':
      return {
        kind: 'danger',
        title: 'Chosen cluster cannot provide app updates',
        // If managing cluster cannot provide updates because it's unreachable,
        // the error needs to be shown here, instead of in a separate alert.
        showInlineUnreachableErrors:
          updateSource.options.unreachableClusters.some(
            c => c.clusterUri === updateSource.options.managingClusterUri
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
