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

import { Version } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb';

import { RootClusterUri } from 'teleterm/ui/uri';

export async function resolveUpdateSource(sources: {
  getEnvVars(): {
    TELEPORT_CDN_BASE_URL: string;
    TELEPORT_TOOLS_VERSION: string;
  };
  getAutoUpdate(): Promise<Version[]>;
  getManagedVersion(): string;
}): Promise<UpdateSource> {
  const envVars = sources.getEnvVars();
  const cdnBaseUrl = envVars.TELEPORT_CDN_BASE_URL;
  if (!cdnBaseUrl) {
    return { resolved: false, reason: 'no-cdn-set' };
  }
  const envVarValue = envVars.TELEPORT_TOOLS_VERSION;
  if (envVarValue === 'off') {
    return { resolved: false, reason: 'disabled-by-env-var' };
  }
  if (envVarValue) {
    return {
      resolved: true,
      version: envVarValue,
      cdnBaseUrl,
      source: { kind: 'env-var' },
    };
  }

  const versions = (await sources.getAutoUpdate()).filter(
    v => v.toolsAutoUpdate
  );
  console.log(versions);
  if (!versions.length) {
    return {
      resolved: false,
      reason: 'no-cluster-with-autoupdate',
    };
  }

  const availableVersions = versions.map(v => ({
    clusterUri: v.clusterUri,
    version: v.toolsVersion,
    minSupportedVersion: v.minToolsVersion,
  }));
  const groupedByVersion = Object.groupBy(
    versions,
    ({ toolsVersion }) => toolsVersion
  );
  const mostCompatible = findMostCompatibleVersion(versions);
  if (mostCompatible) {
    return {
      resolved: true,
      source: {
        kind: 'most-compatible',
        availableVersions,
        clusterUris: groupedByVersion[mostCompatible].map(r => r.clusterUri),
      },
      version: mostCompatible,
      cdnBaseUrl,
    };
  }

  const clusterUri = sources.getManagedVersion();
  if (clusterUri) {
    const version = versions.find(v => v.clusterUri === clusterUri);
    if (version) {
      return {
        resolved: true,
        source: {
          kind: 'cluster-override',
          availableVersions,
          clusterUri,
        },
        version: version.toolsVersion,
        cdnBaseUrl,
      };
    }
  }

  return {
    resolved: false,
    reason: 'conflicting-cluster-versions',
    availableVersions,
  };
}

function findMostCompatibleVersion(versions: Version[]): string | undefined {
  const candidates = Array.from(new Set(versions.map(s => s.toolsVersion)));

  // Sort candidates in descending order (newest first)
  candidates.sort(compareVersions);

  for (const version of candidates) {
    const N = getMajor(version);
    const Nplus1 = N + 1;

    const compatible = versions.every(({ minToolsVersion, toolsVersion }) => {
      const min = getMajor(minToolsVersion);
      const max = getMajor(toolsVersion);
      return (N >= min && N <= max) || (Nplus1 >= min && Nplus1 <= max);
    });

    if (compatible) {
      return version;
    }
  }

  return undefined;
}

export type ResolvedUpdateSource = {
  resolved: true;
  version: string;
  cdnBaseUrl: string;
  source:
    | {
        /** TELEPORT_TOOLS_VERSION configures app version. */
        kind: 'env-var';
      }
    | {
        kind: 'most-compatible';
        clusterUris: RootClusterUri[]; // multiple clusters may specify the same version.
        availableVersions: AvailableVersion[];
      }
    | {
        kind: 'cluster-override';
        clusterUri: RootClusterUri;
        availableVersions: AvailableVersion[];
      };
};

export type UnresolvedUpdate =
  | {
      resolved: false;
      reason:
        | 'disabled-by-env-var'
        | 'no-cdn-set'
        | 'no-cluster-with-autoupdate';
    }
  | {
      resolved: false;
      reason: 'conflicting-cluster-versions';
      availableVersions: AvailableVersion[];
    };

export type UpdateSource = ResolvedUpdateSource | UnresolvedUpdate;

export interface AvailableVersion {
  clusterUri: RootClusterUri;
  version: string;
  minSupportedVersion: string;
}

function parseSemver(version: string) {
  return version.split('.').map(n => parseInt(n, 10));
}

function getMajor(version: string) {
  return parseSemver(version)[0];
}

function compareVersions(a: string, b: string): number {
  const [aMaj, aMin, aPatch] = parseSemver(a);
  const [bMaj, bMin, bPatch] = parseSemver(b);

  if (aMaj !== bMaj) return bMaj - aMaj;
  if (aMin !== bMin) return bMin - aMin;
  return bPatch - aPatch;
}
