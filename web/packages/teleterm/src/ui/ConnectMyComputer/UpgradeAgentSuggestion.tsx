/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Alert, Text } from 'design';
import Link from 'design/Link';

import { compareSemVers } from 'shared/utils/semVer';

import { RuntimeSettings } from 'teleterm/mainProcess/types';

import { isAgentCompatible } from './CompatibilityPromise';

export function shouldShowAgentUpgradeSuggestion(
  proxyVersion: string,
  runtimeSettings: Pick<RuntimeSettings, 'appVersion' | 'isLocalBuild'>
): boolean {
  const { appVersion, isLocalBuild } = runtimeSettings;
  const isClusterOnOlderVersion =
    compareSemVers(proxyVersion, appVersion) === 1;
  return (
    isAgentCompatible(proxyVersion, runtimeSettings) &&
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
        upgrading it to {props.proxyVersion} by updating Teleport Connect. Visit{' '}
        <Link href="https://goteleport.com/download" target="_blank">
          the downloads page
        </Link>
        .
      </Text>
    </Alert>
  );
}
