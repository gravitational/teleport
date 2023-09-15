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
import { Text, ButtonPrimary, Alert } from 'design';

import Link from 'design/Link';

import { compareSemVers } from 'shared/utils/semVer';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { Cluster } from 'teleterm/services/tshd/types';
import { RuntimeSettings } from 'teleterm/mainProcess/types';

import { useConnectMyComputerContext } from './connectMyComputerContext';

const CONNECT_MY_COMPUTER_RELEASE = '14.1.0';

export function isAgentCompatible(
  cluster: Cluster,
  runtimeSettings: RuntimeSettings
): boolean {
  if (cluster.serverVersion === '') {
    return false;
  }
  if (runtimeSettings.isLocalBuild) {
    return true;
  }
  const majorAppVersion = getMajorVersion(runtimeSettings.appVersion);
  const majorClusterVersion = getMajorVersion(cluster.serverVersion);
  return (
    majorAppVersion === majorClusterVersion ||
    majorAppVersion === majorClusterVersion - 1 // app one major version behind the cluster
  );
}

export function CompatibilityError(): JSX.Element {
  const ctx = useAppContext();
  const workspaceContext = useWorkspaceContext();
  const cluster = ctx.clustersService.findCluster(
    workspaceContext.rootClusterUri
  );

  const { serverVersion } = cluster;
  const clusterMajorVersion = getMajorVersion(serverVersion);
  const { appVersion } = ctx.mainProcessClient.getRuntimeSettings();
  const appMajorVersion = getMajorVersion(appVersion);
  const connectMyComputerReleaseMajorVersion = getMajorVersion(
    CONNECT_MY_COMPUTER_RELEASE
  );

  // offer a downgrade only to a release that has 'Connect My Computer'
  const isAppDowngradePossible =
    clusterMajorVersion >= connectMyComputerReleaseMajorVersion;
  const downgradeAppTo =
    clusterMajorVersion === connectMyComputerReleaseMajorVersion
      ? CONNECT_MY_COMPUTER_RELEASE
      : `${clusterMajorVersion}.x.x`;

  let $content: JSX.Element;
  if (appMajorVersion > clusterMajorVersion) {
    $content = (
      <>
        , clusters don't support clients that are on a newer major version. If
        you wish to connect your computer,{' '}
        {isAppDowngradePossible && (
          <>downgrade the app to {downgradeAppTo} or </>
        )}
        upgrade the cluster to version {appMajorVersion}.x.x.
      </>
    );
  }
  if (appMajorVersion < clusterMajorVersion) {
    $content = (
      <>
        , clusters don't support clients that are more than one major version
        behind. If you wish to connect your computer, upgrade the app to{' '}
        {clusterMajorVersion}.x.x.
      </>
    );
  }

  return (
    <>
      <Alert>Detected an incompatible agent version.</Alert>
      <Text>
        The cluster is on version {serverVersion} while Teleport Connect is on
        version {appVersion}. Per our{' '}
        <Link
          href="https://goteleport.com/docs/faq/#version-compatibility"
          target="_blank"
        >
          compatibility promise
        </Link>
        {$content}
      </Text>
      <ButtonPrimary
        mt={3}
        mx="auto"
        type="button"
        as="a"
        style={{ display: 'flex', width: 'fit-content' }}
        href="https://goteleport.com/download"
        target="_blank"
      >
        Visit the downloads page
      </ButtonPrimary>
    </>
  );
}

export function UpgradeAgentSuggestion(): JSX.Element {
  const ctx = useAppContext();
  const workspaceContext = useWorkspaceContext();
  const { isNonCompatibleAgent } = useConnectMyComputerContext();
  const cluster = ctx.clustersService.findCluster(
    workspaceContext.rootClusterUri
  );
  const { serverVersion } = cluster;
  const { appVersion, isLocalBuild } =
    ctx.mainProcessClient.getRuntimeSettings();

  const isClusterAlreadyOnNewerVersion =
    compareSemVers(appVersion, serverVersion) === 1;
  if (isNonCompatibleAgent || isClusterAlreadyOnNewerVersion || isLocalBuild) {
    return null;
  }

  return (
    <Alert kind="info">
      <Text>
        The agent is running version {appVersion} of Teleport. Consider
        upgrading it to {serverVersion} by updating Teleport Connect.{' '}
        <Link href="https://goteleport.com/download" target="_blank">
          Visit the downloads page.
        </Link>
      </Text>
    </Alert>
  );
}

function getMajorVersion(version: string): number {
  return parseInt(version.split('.')[0]);
}
