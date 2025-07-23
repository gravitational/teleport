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

import type {
  ClusterVersionInfo,
  GetClusterVersionsResponse,
  UnreachableCluster,
} from 'gen-proto-ts/teleport/lib/teleterm/auto_update/v1/auto_update_service_pb';
import { compare, major } from 'shared/utils/semVer';

import Logger from 'teleterm/logger';
import { RootClusterUri } from 'teleterm/ui/uri';

const logger = new Logger('resolveAutoUpdatesStatus');

/**
 * Determines the auto-update status based on all connected clusters.
 * Updates can be either enabled or disabled, depending on the passed sources.
 * The version is selected using the following precedence:
 * 1. `TELEPORT_TOOLS_VERSION` env var, if defined.
 * 2. `tools_version` from the cluster manually selected to manage updates, if selected.
 * 3. Most compatible version, if found.
 * 4. If there's no version at this point, stop auto updates.
 */
export async function resolveAutoUpdatesStatus(sources: {
  versionEnvVar: string;
  managingClusterUri: string | undefined;
  getClusterVersions(): Promise<GetClusterVersionsResponse>;
}): Promise<AutoUpdatesStatus> {
  if (sources.versionEnvVar === 'off') {
    return { enabled: false, reason: 'disabled-by-env-var' };
  }
  if (sources.versionEnvVar) {
    return {
      enabled: true,
      version: sources.versionEnvVar,
      source: { kind: 'env-var' },
    };
  }

  const clusterVersions = await sources.getClusterVersions();
  return findVersionFromClusters({
    clusterVersions,
    managingClusterUri: sources.managingClusterUri,
  });
}

/**
 * Attempts to find tools version from a user-selected cluster or connected
 * clusters.
 */
function findVersionFromClusters(sources: {
  managingClusterUri: string | undefined;
  clusterVersions: GetClusterVersionsResponse;
}): AutoUpdatesStatus {
  const { reachableClusters, unreachableClusters } = sources.clusterVersions;
  const clusters = makeClusters(reachableClusters);
  const autoUpdateCandidateClusters = clusters.filter(c => c.toolsAutoUpdate);
  if (!autoUpdateCandidateClusters.length) {
    return {
      enabled: false,
      reason: 'no-cluster-with-auto-update',
      clusters,
      unreachableClusters,
    };
  }

  const { managingClusterUri } = sources;
  const managingCluster = clusters.find(
    c => c.clusterUri === managingClusterUri
  );

  if (managingCluster?.toolsAutoUpdate) {
    return {
      enabled: true,
      version: managingCluster.toolsVersion,
      source: {
        kind: 'managing-cluster',
        clusters,
        clusterUri: managingCluster.clusterUri,
        unreachableClusters,
      },
    };
  }

  const managingClusterIsSkipped =
    !!managingCluster ||
    unreachableClusters.some(c => c.clusterUri === managingClusterUri);

  if (managingClusterIsSkipped) {
    logger.warn(
      `Cluster ${managingClusterUri} managing updates is unreachable or not managing updates, continuing resolution with most compatible version.`
    );
  }

  const mostCompatibleVersion = findMostCompatibleToolsVersion(clusters);
  if (mostCompatibleVersion) {
    return {
      enabled: true,
      version: mostCompatibleVersion,
      source: {
        kind: 'most-compatible',
        clusters,
        unreachableClusters,
        skippedManagingClusterUri: managingClusterIsSkipped
          ? managingClusterUri
          : '',
      },
    };
  }

  return {
    enabled: false,
    reason: 'no-compatible-version',
    unreachableClusters,
    clusters,
  };
}

/** When `false` is returned, the user will need to click 'Download' manually. */
export function shouldAutoDownload(updatesStatus: AutoUpdatesEnabled): boolean {
  const { kind } = updatesStatus.source;
  switch (kind) {
    case 'env-var':
    case 'managing-cluster':
      return true;
    case 'most-compatible':
      return (
        // Prevent auto-downloading in cases where the most-compatible approach had
        // to ignore some unreachable clusters.
        // This can happen when a cluster that manages updates becomes temporarily unavailable
        // – we don't want another cluster to suddenly take over managing updates.
        updatesStatus.source.unreachableClusters.length === 0 &&
        // Do not download the most compatible version automatically if the managing
        // cluster is unreachable or has auto-updates disabled.
        !updatesStatus.source.skippedManagingClusterUri
      );
    default:
      kind satisfies never;
  }
}

/** Assigns each cluster a compatibility with client tools from other clusters. */
function makeClusters(versions: ClusterVersionInfo[]): Cluster[] {
  return versions.map(version => {
    const majorToolsVersion = major(version.toolsVersion);

    const otherCompatibleClusters = versions
      .filter(v => {
        // The list should only contain other clusters.
        if (v.clusterUri === version.clusterUri) {
          return false;
        }

        const minMajor = major(v.minToolsVersion);
        const maxMajor = major(v.toolsVersion);

        // Check compatibility: cluster's major version must be within [minMajor, maxMajor].
        return majorToolsVersion >= minMajor && majorToolsVersion <= maxMajor;
      })
      .map(v => v.clusterUri);

    return {
      ...version,
      otherCompatibleClusters,
    };
  });
}

/** Finds the highest version of client tools that is compatible with all clusters. */
function findMostCompatibleToolsVersion(
  clusters: Cluster[]
): string | undefined {
  // Get the highest version first.
  const sortedAutoUpdateCandidates = clusters
    .filter(c => c.toolsAutoUpdate)
    .toSorted((a, b) => compare(b.toolsVersion, a.toolsVersion));

  const allClusters = new Set(clusters.map(c => c.clusterUri));

  for (const candidate of sortedAutoUpdateCandidates) {
    // The candidate is compatible with itself and `otherCompatibleClusters`.
    const compatibleClusters = new Set([
      candidate.clusterUri,
      ...candidate.otherCompatibleClusters,
    ]);
    // Check if candidate is compatible with all required clusters.
    if (compatibleClusters.isSupersetOf(allClusters)) {
      return candidate.toolsVersion;
    }
  }
}

export type AutoUpdatesEnabled = {
  enabled: true;
  version: string;
  source:
    | {
        /** TELEPORT_TOOLS_VERSION configures app version. */
        kind: 'env-var';
      }
    | {
        /** Updates are auto by a specific cluster. */
        kind: 'managing-cluster';
        /** URI of the managing cluster. */
        clusterUri: RootClusterUri;
        /** Clusters considered during version resolution. */
        clusters: Cluster[];
        /** Clusters from which version information could not be retrieved. */
        unreachableClusters: UnreachableCluster[];
      }
    | {
        /** Updates determined by the most compatible clusters available. */
        kind: 'most-compatible';
        /** Clusters considered during version resolution. */
        clusters: Cluster[];
        /**
         * Clusters from which version information could not be retrieved.
         * If non-empty, the update is not automatically downloaded.
         * */
        unreachableClusters: UnreachableCluster[];
        /**
         * Indicates whether a cluster configured to manage updates is unreachable
         * or not managing updates (toolsAutoUpdate: false).
         */
        skippedManagingClusterUri: string;
      };
};

export type AutoUpdatesDisabled =
  | {
      enabled: false;
      /** `TELEPORT_TOOLS_VERSION` is 'off'. */
      reason: 'disabled-by-env-var';
    }
  | {
      enabled: false;
      /** There is no cluster that could manage updates. */
      reason: 'no-cluster-with-auto-update';
      /** Clusters considered during version resolution. */
      clusters: Cluster[];
      /** Clusters from which version information could not be retrieved. */
      unreachableClusters: UnreachableCluster[];
    }
  | {
      enabled: false;
      /**
       * There are clusters that could manage updates, but they specify
       * incompatible client tools versions.
       */
      reason: 'no-compatible-version';
      /** Clusters considered during version resolution. */
      clusters: Cluster[];
      /** Clusters from which version information could not be retrieved. */
      unreachableClusters: UnreachableCluster[];
    };

export type AutoUpdatesStatus = AutoUpdatesEnabled | AutoUpdatesDisabled;

/** Represents a cluster that could manage updates. */
export interface Cluster {
  /** URI of the cluster. */
  clusterUri: RootClusterUri;
  /** Whether the client should automatically update the tools version. */
  toolsAutoUpdate: boolean;
  /** Tools version required by this cluster. */
  toolsVersion: string;
  /** Minimum tools version allowed by this cluster. */
  minToolsVersion: string;
  /** URIs of clusters whose client tools are compatible with this cluster's client tools. */
  otherCompatibleClusters: RootClusterUri[];
}
