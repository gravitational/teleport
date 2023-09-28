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

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { RuntimeSettings } from 'teleterm/mainProcess/types';

const CONNECT_MY_COMPUTER_RELEASE_VERSION = '14.1.0';
const CONNECT_MY_COMPUTER_RELEASE_MAJOR_VERSION = 14;

export type AgentCompatibility = 'unknown' | 'compatible' | 'incompatible';

export function checkAgentCompatibility(
  proxyVersion: string,
  runtimeSettings: Pick<RuntimeSettings, 'appVersion' | 'isLocalBuild'>
): AgentCompatibility {
  // The proxy version is not immediately available
  // (it requires fetching a cluster with details).
  // Because of that, we have to return 'unknown' when we do not yet know it.
  if (!proxyVersion) {
    return 'unknown';
  }
  if (runtimeSettings.isLocalBuild) {
    return 'compatible';
  }
  const majorAppVersion = getMajorVersion(runtimeSettings.appVersion);
  const majorClusterVersion = getMajorVersion(proxyVersion);
  return majorAppVersion === majorClusterVersion ||
    majorAppVersion === majorClusterVersion - 1 // app one major version behind the cluster
    ? 'compatible'
    : 'incompatible';
}

export function CompatibilityError(props: {
  hideAlert?: boolean;
}): JSX.Element {
  const { proxyVersion, appVersion } = useVersions();

  const clusterMajorVersion = getMajorVersion(proxyVersion);
  const appMajorVersion = getMajorVersion(appVersion);

  // offer a downgrade only to a release that has 'Connect My Computer'
  // DELETE IN 17.0.0 (gzdunek): by the time 17.0 releases, 14.x will no longer be
  // supported and then downgradeAppTo will simply become ${clusterMajorVersion}.x.x,
  // and we will not have to check if downgrade is possible.
  const isAppDowngradePossible =
    clusterMajorVersion >= CONNECT_MY_COMPUTER_RELEASE_MAJOR_VERSION;
  const downgradeAppTo =
    clusterMajorVersion === CONNECT_MY_COMPUTER_RELEASE_MAJOR_VERSION
      ? CONNECT_MY_COMPUTER_RELEASE_VERSION
      : `${clusterMajorVersion}.x.x`;

  let $content: JSX.Element;
  if (appMajorVersion > clusterMajorVersion) {
    $content = (
      <>
        , clusters don't support clients that are on a newer major version. To
        use Connect My Computer,{' '}
        {isAppDowngradePossible && (
          <>downgrade the app to version {downgradeAppTo} or </>
        )}
        upgrade the cluster to version {appMajorVersion}.x.x.
      </>
    );
  }
  if (appMajorVersion < clusterMajorVersion) {
    $content = (
      <>
        , clusters don't support clients that are more than one major version
        behind. To use Connect My Computer, upgrade the app to{' '}
        {clusterMajorVersion}.x.x.
      </>
    );
  }

  return (
    <>
      {!props.hideAlert && (
        <Alert>
          The agent version is not compatible with the cluster version
        </Alert>
      )}
      <Text>
        The cluster is on version {proxyVersion} while Teleport Connect is on
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

export function useVersions() {
  const ctx = useAppContext();
  const workspaceContext = useWorkspaceContext();
  const cluster = ctx.clustersService.findCluster(
    workspaceContext.rootClusterUri
  );
  const { proxyVersion } = cluster;
  const { appVersion, isLocalBuild } =
    ctx.mainProcessClient.getRuntimeSettings();

  return { proxyVersion, appVersion, isLocalBuild };
}

function getMajorVersion(version: string): number {
  return parseInt(version.split('.')[0]);
}
