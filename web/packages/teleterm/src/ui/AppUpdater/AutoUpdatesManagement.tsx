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
import Flex, { Stack } from 'design/Flex';
import { Cog } from 'design/Icon';
import Link from 'design/Link';
import { RadioGroup } from 'design/RadioGroup';
import { H3, P3 } from 'design/Text';
import { pluralize } from 'shared/utils/text';

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

  const getClusterName = clusterNameGetter(props.clusterGetter);
  const content =
    status.enabled === true
      ? makeContentForEnabledAutoUpdates(status, getClusterName)
      : makeContentForDisabledAutoUpdates(status);
  const retryButton = {
    content: 'Retry',
    onClick: props.onCheckForUpdates,
    disabled: props.updateEventKind === 'download-progress',
  };

  return (
    <>
      {content && (
        <Alert
          width="100%"
          mb={0}
          icon={content.kind === 'neutral' ? Cog : undefined}
          kind={content.kind}
          details={content.description}
          primaryAction={content.showRetry && retryButton}
        >
          {'title' in content ? content.title : ''}
        </Alert>
      )}
      <ManagingClusterSelector
        autoUpdatesStatus={status}
        changeManagingCluster={props.changeManagingCluster}
        isCheckingForUpdates={props.updateEventKind === 'checking-for-update'}
        getClusterName={getClusterName}
        onRetry={props.onCheckForUpdates}
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
  onRetry,
}: {
  autoUpdatesStatus: AutoUpdatesStatus;
  isCheckingForUpdates: boolean;
  changeManagingCluster(clusterUri: RootClusterUri | undefined): void;
  getClusterName(clusterUri: RootClusterUri): string;
  onRetry(): void;
}) {
  // Allows optimistic UI updates without waiting for autoUpdatesStatus.
  const [optimisticManagingCluster, setOptimisticManagingCluster] = useState<
    '' | RootClusterUri
  >(autoUpdatesStatus.options.managingClusterUri || '');

  const options = makeOptions({
    status: autoUpdatesStatus,
    getClusterName,
    disabled: isCheckingForUpdates,
    highestCompatibleVersion:
      autoUpdatesStatus.options.highestCompatibleVersion,
    onRetry,
  });

  return (
    <>
      {/* Two because there is always 'Use highest compatible version' option */}
      {(options.length > 2 || autoUpdatesStatus.options.managingClusterUri) && (
        <Stack
          width="100%"
          gap={2}
          p={3}
          borderRadius={2}
          css={`
            background-color: ${p =>
              p.theme.colors.interactive.tonal.neutral[0]};
          `}
        >
          <Stack gap={0}>
            <H3>Updates source</H3>
            <P3>Choose which cluster to follow for updates.</P3>
          </Stack>
          <RadioGroup
            gap={2}
            name="managingCluster"
            size="small"
            value={optimisticManagingCluster}
            onChange={clusterUri => {
              setOptimisticManagingCluster(clusterUri);
              changeManagingCluster(clusterUri || undefined);
            }}
            options={options}
          />
        </Stack>
      )}
    </>
  );
}

function makeOptions({
  status,
  getClusterName,
  highestCompatibleVersion,
  disabled,
  onRetry,
}: {
  status: AutoUpdatesStatus;
  getClusterName: (clusterUri: RootClusterUri) => string;
  disabled: boolean;
  highestCompatibleVersion: string;
  onRetry(): void;
}) {
  const highestCompatible = {
    label: 'Use the highest compatible version from your clusters',
    helperText: !highestCompatibleVersion ? (
      <TextWithWarning text="No cluster provides a version compatible with all other clusters." />
    ) : (
      `Teleport Connect ${highestCompatibleVersion} · Compatible with all clusters.`
    ),
    disabled: disabled,
    value: '',
  };

  const candidateClusters = status.options.clusters
    .filter(c => c.toolsAutoUpdate)
    .map(c => {
      const otherCompatibleClusters = c.otherCompatibleClusters.map(c =>
        getClusterName(c)
      );
      const compatibility = otherCompatibleClusters.length
        ? `Also compatible with ${pluralize(otherCompatibleClusters.length, 'cluster')} ${listFormatter.format(otherCompatibleClusters.toSorted())}.`
        : '';

      return {
        disabled,
        label: getClusterName(c.clusterUri),
        helperText: [`Teleport Connect ${c.toolsVersion}`, compatibility]
          .filter(Boolean)
          .join(' · '),
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
            Teleport Connect {c.toolsVersion}
            <br />
            <TextWithWarning text="Automatic client tools updates are disabled on this cluster." />
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
        <UnreachableClusterHelper
          onRetry={onRetry}
          error={cluster.errorMessage}
        />
      ),
      value: cluster.clusterUri,
    })
  );

  return [
    highestCompatible,
    ...candidateClusters,
    ...nonCandidateClusters,
    ...unreachableClusters,
  ];
}

function TextWithWarning(props: { text: string }) {
  return (
    <span>
      <span
        css={`
          color: ${props => props.theme.colors.error.main};
        `}
      >
        ⚠︎︎
      </span>{' '}
      {props.text}
    </span>
  );
}

function UnreachableClusterHelper(props: { error: string; onRetry(): void }) {
  const [showsMore, setShowsMore] = useState(false);

  return (
    <Stack
      alignItems="start"
      justifyContent="space-between"
      width="100%"
      gap={0}
    >
      <TextWithWarning text="Version unavailable · Cluster is unreachable." />
      {showsMore ? <span>{props.error}</span> : ''}
      <Flex gap={1}>
        {/* These buttons unfortunately trigger hover of the entire radio option */}
        {/* because they are inside the label element.*/}
        <Link
          onClick={e => {
            e.stopPropagation();
            e.preventDefault();
            setShowsMore(v => !v);
          }}
        >
          {showsMore ? 'Show Less' : 'Show More'}
        </Link>
        <Link
          onClick={e => {
            e.stopPropagation();
            e.preventDefault();
            props.onRetry();
          }}
        >
          Retry
        </Link>
      </Flex>
    </Stack>
  );
}

function makeContentForEnabledAutoUpdates(
  status: AutoUpdatesEnabled,
  getClusterName: (clusterUri: RootClusterUri) => string
): {
  description: string;
  kind: 'neutral' | 'warning';
  showRetry?: boolean;
} {
  switch (status.source) {
    case 'env-var':
      return {
        kind: 'neutral',
        description: `The app is set to stay on version ${status.version} by your device settings.`,
      };
    case 'managing-cluster':
      return;
    case 'highest-compatible':
      const providingClusters = status.options.clusters
        .filter(c => c.toolsAutoUpdate && c.toolsVersion === status.version)
        .map(c => getClusterName(c.clusterUri));
      // Show info if there's only one cluster.
      if (
        status.options.clusters.length === 1 &&
        status.options.unreachableClusters.length === 0
      ) {
        return {
          kind: 'neutral',
          description: `App updates are managed by the cluster ${providingClusters}, which requires client version ${status.version}.`,
        };
      }
  }
}

function makeContentForDisabledAutoUpdates(updateSource: AutoUpdatesDisabled): {
  title?: string;
  description?: ReactNode;
  kind: 'danger' | 'neutral';
  showRetry?: boolean;
} {
  switch (updateSource.reason) {
    case 'disabled-by-env-var':
      return {
        kind: 'neutral',
        description: 'App updates are disabled by your device settings.',
      };
    case 'no-cluster-with-auto-update':
      // There's only one cluster and it's unreachable, show error inline.
      if (
        updateSource.options.unreachableClusters.length === 1 &&
        updateSource.options.clusters.length === 0
      ) {
        return {
          kind: 'danger',
          title: 'App updates are disabled, the cluster is unreachable',
          description:
            updateSource.options.unreachableClusters.at(0).errorMessage,
          showRetry: true,
        };
      }
      // There is no cluster with updates enabled and some clusters cannot be reached.
      if (updateSource.options.unreachableClusters.length > 1) {
        return {
          kind: 'danger',
          title: 'App updates are disabled.',
        };
      }
      // All clusters have updates disabled.
      return {
        kind: 'neutral',
        title: 'App updates are disabled',
        description: (
          <>
            The cluster needs to{' '}
            <Link href="https://goteleport.com/docs/upgrading/automatic-updates">
              enroll in automatic updates
            </Link>{' '}
            to keep Teleport Connect updated.
          </>
        ),
      };
    case 'managing-cluster-unable-to-manage':
      return {
        kind: 'danger',
        title: 'App updates are disabled.',
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
