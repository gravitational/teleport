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
 * 3. Highest compatible version, if found.
 * 4. If there's no version at this point, stop auto updates.
 */
export async function resolveAutoUpdatesStatus(sources: {
  versionEnvVar: string;
  managingClusterUri: string | undefined;
  getClusterVersions(): Promise<GetClusterVersionsResponse>;
}): Promise<AutoUpdatesStatus> {
  if (sources.versionEnvVar === 'off') {
    return {
      enabled: false,
      reason: 'disabled-by-env-var',
      options: {
        clusters: [],
        unreachableClusters: [],
        highestCompatibleVersion: '',
        managingClusterUri: '',
      },
    };
  }
  if (sources.versionEnvVar) {
    return {
      enabled: true,
      version: sources.versionEnvVar,
      source: 'env-var',
      options: {
        clusters: [],
        unreachableClusters: [],
        highestCompatibleVersion: '',
        managingClusterUri: '',
      },
    };
  }

  const clusterVersions = await sources.getClusterVersions();
  const options = createAutoUpdateOptions({
    managingClusterUri: sources.managingClusterUri,
    clusterVersions,
  });
  return findVersionFromClusters(options);
}

function createAutoUpdateOptions(sources: {
  managingClusterUri: string | undefined;
  clusterVersions: GetClusterVersionsResponse;
}): AutoUpdatesOptions {
  const { reachableClusters, unreachableClusters } = sources.clusterVersions;
  const clusters = makeClusters(reachableClusters);
  const highestCompatibleVersion = findMostCompatibleToolsVersion(clusters);
  // If the managing cluster URI doesn't exist within connected clusters, ignore it completely.
  // The client version cannot be determined in this case.
  // Additionally, a cluster not listed in the connected clusters should not attempt
  // to manage versions for them.
  const managingClusterExists =
    clusters.some(c => c.clusterUri === sources.managingClusterUri) ||
    unreachableClusters.some(c => c.clusterUri === sources.managingClusterUri);

  return {
    clusters,
    unreachableClusters,
    highestCompatibleVersion,
    managingClusterUri: managingClusterExists
      ? sources.managingClusterUri
      : undefined,
  };
}

/**
 * Attempts to find tools version from a user-selected cluster or connected
 * clusters.
 */
function findVersionFromClusters(
  options: AutoUpdatesOptions
): AutoUpdatesStatus {
  const { managingClusterUri, clusters, highestCompatibleVersion } = options;

  if (managingClusterUri) {
    const managingCluster = clusters.find(
      c => c.clusterUri === managingClusterUri
    );

    if (managingCluster?.toolsAutoUpdate) {
      return {
        options,
        enabled: true,
        version: managingCluster.toolsVersion,
        source: 'managing-cluster',
      };
    }

    logger.warn(
      `Cluster ${managingClusterUri} managing updates is unreachable or not managing updates.`
    );
    return {
      options,
      enabled: false,
      reason: 'managing-cluster-unable-to-manage',
    };
  }

  const autoUpdateCandidateClusters = clusters.filter(c => c.toolsAutoUpdate);
  if (!autoUpdateCandidateClusters.length) {
    return {
      options,
      enabled: false,
      reason: 'no-cluster-with-auto-update',
    };
  }

  if (highestCompatibleVersion) {
    return {
      options,
      enabled: true,
      version: highestCompatibleVersion,
      source: 'highest-compatible',
    };
  }

  return {
    options,
    enabled: false,
    reason: 'no-compatible-version',
  };
}

/** When `false` is returned, the user will need to click 'Download' manually. */
export function shouldAutoDownload(updatesStatus: AutoUpdatesEnabled): boolean {
  const { source } = updatesStatus;
  switch (source) {
    case 'env-var':
    case 'managing-cluster':
      return true;
    case 'highest-compatible':
      return (
        // Prevent auto-downloading in cases where the highest-compatible approach had
        // to ignore some unreachable clusters.
        // Since compatibility can't be verified against those clusters,
        // the selected version might be incompatible with them.
        // If the clusters are temporarily unavailable, a future update check may
        // revert to a previously compatible version.
        // To avoid these version switches, the decision whether to install the
        // update is left to the user.
        updatesStatus.options.unreachableClusters.length === 0
      );
    default:
      source satisfies never;
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

export interface AutoUpdatesEnabled {
  enabled: true;
  version: string;
  /**
   * Source of the update:
   * - `env-var` - TELEPORT_TOOLS_VERSION configures app version.
   * - `managing-cluster` - updates are managed by a manually configured cluster.
   * - `highest-compatible` - updates are determined by the highest compatible version available.
   */
  source: 'env-var' | 'managing-cluster' | 'highest-compatible';
  /**
   * Represents the options considered during the auto-update version resolution process.
   * If updates are configured via the environment variable, all fields will be empty or undefined.
   */
  options: AutoUpdatesOptions;
}

export interface AutoUpdatesDisabled {
  enabled: false;
  /**
   * Reason the updates are disabled:
   * `disabled-by-env-var` - `TELEPORT_TOOLS_VERSION` is 'off'.
   * `managing-cluster-unable-to-manage` - the manually selected managing cluster is either
   * unreachable or it has since disabled autoupdates.
   * `no-cluster-with-auto-update` - there is no cluster that could manage updates.
   * `no-compatible-version` - there are clusters that could manage updates, but
   * they specify incompatible client tools versions.
   */
  reason:
    | 'disabled-by-env-var'
    | 'managing-cluster-unable-to-manage'
    | 'no-cluster-with-auto-update'
    | 'no-compatible-version';
  /**
   * Represents the options considered during the auto-update version resolution process.
   * If updates are disabled via the environment variable, all fields will be empty or undefined.
   */
  options: AutoUpdatesOptions;
}

export interface AutoUpdatesOptions {
  /** The highest version that is compatible with all other clusters. */
  highestCompatibleVersion: string | undefined;
  /** URI of a manually selected cluster to manage updates. */
  managingClusterUri: string | undefined;
  /** Clusters considered during version resolution. */
  clusters: Cluster[];
  /** Clusters from which version information could not be retrieved. */
  unreachableClusters: UnreachableCluster[];
}

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
