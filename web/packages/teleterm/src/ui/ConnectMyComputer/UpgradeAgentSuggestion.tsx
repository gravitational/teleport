/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import type { JSX } from 'react';

import { Alert, Text } from 'design';
import Link from 'design/Link';
import { compareSemVers } from 'shared/utils/semVer';

import { RuntimeSettings } from 'teleterm/mainProcess/types';

import { checkAgentCompatibility } from './CompatibilityPromise';

export function shouldShowAgentUpgradeSuggestion(
  proxyVersion: string,
  runtimeSettings: Pick<RuntimeSettings, 'appVersion' | 'isLocalBuild'>
): boolean {
  const { appVersion, isLocalBuild } = runtimeSettings;
  const isClusterOnOlderVersion =
    compareSemVers(proxyVersion, appVersion) === 1;
  return (
    checkAgentCompatibility(proxyVersion, runtimeSettings) === 'compatible' &&
    isClusterOnOlderVersion &&
    !isLocalBuild
  );
}

export function UpgradeAgentSuggestion(props: {
  proxyVersion: string;
  appVersion: string;
}): JSX.Element {
  return (
    <Alert kind="info">
      <Text>
        The agent is running version {props.appVersion} of Teleport. Consider
        upgrading it to version {props.proxyVersion} by updating Teleport
        Connect. Visit{' '}
        <Link href="https://goteleport.com/download" target="_blank">
          the downloads page
        </Link>
        .
      </Text>
    </Alert>
  );
}
