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

import { major } from 'shared/utils/semVer';

import useTeleport from 'teleport/useTeleport';

export function useClusterVersion() {
  const ctx = useTeleport();
  const clusterVersion = ctx.storeUser.getClusterAuthVersion();
  const clusterVersionMajor = major(clusterVersion);

  const _diff = (version?: string) => diff(version, clusterVersionMajor);

  return {
    clusterVersion,
    diff: _diff,
  };
}

export type ClusterVersionDiff = ReturnType<typeof diff>;

function diff(
  version: string | null | undefined,
  clusterVersionMajor: number | null | undefined
) {
  if (
    !version ||
    clusterVersionMajor === undefined ||
    clusterVersionMajor === null
  )
    return null;
  const versionMajor = major(version);
  switch (true) {
    case versionMajor === clusterVersionMajor + 2:
      return 'n+2';
    case versionMajor === clusterVersionMajor + 1:
      return 'n+1';
    case versionMajor > clusterVersionMajor:
      return 'n+';
    case versionMajor === clusterVersionMajor:
      return 'n';
    case versionMajor === clusterVersionMajor - 1:
      return 'n-1';
    case versionMajor === clusterVersionMajor - 2:
      return 'n-2';
    case versionMajor < clusterVersionMajor - 2:
      return 'n-';
  }
  return null;
}
