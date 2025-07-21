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
  const candidateClusters = makeCandidateClusters(reachableClusters);
  const autoUpdateCandidateClusters = candidateClusters.filter(
    c => c.toolsAutoUpdate
  );
  if (!autoUpdateCandidateClusters.length) {
    return {
      enabled: false,
      reason: 'no-cluster-with-auto-update',
      candidateClusters,
      unreachableClusters,
    };
  }

  const clusterUri = sources.managingClusterUri;
  if (clusterUri) {
    const cluster = candidateClusters.find(c => c.clusterUri === clusterUri);
    if (cluster?.toolsAutoUpdate) {
      return {
        enabled: true,
        version: cluster.toolsVersion,
        source: {
          kind: 'managing-cluster',
          candidateClusters,
          clusterUri,
          unreachableClusters,
        },
      };
    }
    logger.warn(
      `Cluster ${clusterUri} managing updates was not found, continuing resolution.`
    );
  }

  const mostCompatibleVersion =
    findMostCompatibleToolsVersion(candidateClusters);
  if (mostCompatibleVersion) {
    return {
      enabled: true,
      version: mostCompatibleVersion,
      source: {
        kind: 'most-compatible',
        candidateClusters,
        unreachableClusters,
        clustersUri: candidateClusters
          .filter(c => c.toolsVersion === mostCompatibleVersion)
          .map(c => c.clusterUri),
      },
    };
  }

  return {
    enabled: false,
    reason: 'no-compatible-version',
    unreachableClusters,
    candidateClusters,
  };
}

/** Assigns each candidate a compatibility with client tools from other clusters. */
function makeCandidateClusters(
  versions: ClusterVersionInfo[]
): CandidateCluster[] {
  return versions.map(version => {
    const candidateMajorToolsVersion = major(version.toolsVersion);

    const otherCompatibleClusters = versions
      .filter(v => {
        // The list should only contain other clusters.
        if (v.clusterUri === version.clusterUri) {
          return false;
        }

        const minMajor = major(v.minToolsVersion);
        const maxMajor = major(v.toolsVersion);

        // Check compatibility: candidate's major version must be within [minMajor, maxMajor].
        return (
          candidateMajorToolsVersion >= minMajor &&
          candidateMajorToolsVersion <= maxMajor
        );
      })
      .map(v => v.clusterUri);

    return {
      ...version,
      otherCompatibleClusters,
    };
  });
}

/**
 * Finds the highest version of client tools that is compatible with all
 * candidate clusters.
 */
function findMostCompatibleToolsVersion(
  candidates: CandidateCluster[]
): string | undefined {
  // Get the highest version first.
  const sortedAutoUpdateCandidates = candidates
    .filter(c => c.toolsAutoUpdate)
    .toSorted((a, b) => compare(b.toolsVersion, a.toolsVersion));

  const allClusters = new Set(candidates.map(c => c.clusterUri));

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
        candidateClusters: CandidateCluster[];
        /** Clusters from which version information could not be retrieved. */
        unreachableClusters: UnreachableCluster[];
      }
    | {
        /** Updates determined by the most compatible clusters available. */
        kind: 'most-compatible';
        /** URIs of all clusters that specify the same version. */
        clustersUri: RootClusterUri[];
        /** Clusters considered during version resolution. */
        candidateClusters: CandidateCluster[];
        /**
         * Clusters from which version information could not be retrieved.
         * If non-empty, the update is not automatically downloaded.
         * */
        unreachableClusters: UnreachableCluster[];
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
      candidateClusters: CandidateCluster[];
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
      candidateClusters: CandidateCluster[];
      /** Clusters from which version information could not be retrieved. */
      unreachableClusters: UnreachableCluster[];
    };

export type AutoUpdatesStatus = AutoUpdatesEnabled | AutoUpdatesDisabled;

/** Represents a cluster that could manage updates. */
export interface CandidateCluster {
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
