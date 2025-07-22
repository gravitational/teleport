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

import semver from 'semver';

import useTeleport from 'teleport/useTeleport';

export function useClusterVersion() {
  const ctx = useTeleport();
  const clusterVersion = semver.parse(ctx.storeUser.getClusterAuthVersion());

  const _diff = (version?: string) => diff(version, clusterVersion);

  return {
    clusterVersion,
    diff: _diff,
  };
}

export type ClusterVersionDiff = ReturnType<typeof diff>;

function diff(
  version: string | null | undefined,
  clusterVersion: semver.SemVer | null | undefined
) {
  const parsedVersion = semver.parse(version);
  if (!parsedVersion || !clusterVersion) return null;
  switch (true) {
    case parsedVersion.major === clusterVersion.major + 2:
      return 'n+2';
    case parsedVersion.major === clusterVersion.major + 1:
      return 'n+1';
    case parsedVersion.major > clusterVersion.major:
      return 'n+';
    case parsedVersion.major === clusterVersion.major:
      return 'n';
    case parsedVersion.major === clusterVersion.major - 1:
      return 'n-1';
    case parsedVersion.major === clusterVersion.major - 2:
      return 'n-2';
    case parsedVersion.major < clusterVersion.major - 2:
      return 'n-';
  }
  return null;
}
