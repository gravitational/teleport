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

import { parse, SemVer } from 'shared/utils/semVer';

import useTeleport from 'teleport/useTeleport';

/**
 * **useClusterVersion** returns the cluster (auth) version and a comparison
 * utility (diff). The diff utility can be used to compare the provided
 * component version to the cluster/control-plane version. An indication of
 * cluster compatibility is returned.
 *
 * @returns the cluster (auth) version and a comparison utility (diff)
 */
export function useClusterVersion(): {
  clusterVersion: SemVer | null;
  /**
   * **diff** compares the provided component version to the cluster/control-
   * plane version. An indication of cluster compatibility is returned as
   * follows:
   * - **n**: the component and cluster versions match, including major, minor and patch
   * - **n***: the component and cluster have the same major version, but earlier minor or patch
   * - **n+1**: the component version is one major version ahead of the cluster version
   * - **n+**: the component version is two or more major version ahead of the cluster version
   * - **n-1**: the component version is one major version behind the cluster version
   * - **n-**: the component version is two or more major version behind the cluster version
   *
   * @param version the compare version as string. Must be a full semver
   * version (e.g. "1.2.3"), optionally with a pre-release tag (e.g.
   * "1.2.3-alpha.1") and/or build metadata (e.g. "1.2.3-alpha.1+build.1")
   * @returns an indication of cluster compatibility
   */
  diff: (version?: string) => ClusterVersionDiff | null;
} {
  const ctx = useTeleport();
  const clusterVersionString = ctx.storeUser.getClusterAuthVersion();
  const clusterVersion = parse(clusterVersionString);

  const _diff = (version?: string) => diff(version, clusterVersion);

  return {
    clusterVersion,
    diff: _diff,
  };
}

export type ClusterVersionDiff = ReturnType<typeof diff>;

function diff(
  version: string | null | undefined,
  clusterVersion: SemVer | null
) {
  const v = parse(version);
  if (!v || clusterVersion === null) return null;
  switch (true) {
    case v.major === clusterVersion.major + 1:
      return 'n+1';
    case v.major > clusterVersion.major:
      return 'n+';
    case v.major === clusterVersion.major:
      if (v.compare(clusterVersion) === -1) {
        return 'n*';
      }
      return 'n';
    case v.major === clusterVersion.major - 1:
      return 'n-1';
    case v.major < clusterVersion.major:
      return 'n-';
  }
  return null;
}
