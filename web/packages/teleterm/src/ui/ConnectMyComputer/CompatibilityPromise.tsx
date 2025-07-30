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

import { Alert, ButtonPrimary, Flex, Text } from 'design';
import Link from 'design/Link';

import { RuntimeSettings } from 'teleterm/mainProcess/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';

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

  let $content: JSX.Element;
  if (appMajorVersion > clusterMajorVersion) {
    $content = (
      <>
        , clusters don't support clients that are on a newer major version. To
        use Connect My Computer, downgrade the app to version{' '}
        {clusterMajorVersion}.x.x or upgrade the cluster to version{' '}
        {appMajorVersion}.x.x.
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
    <Flex flexDirection="column" gap={2}>
      {!props.hideAlert && (
        <Alert mb={0}>
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
        mx="auto"
        type="button"
        as="a"
        style={{ display: 'flex', width: 'fit-content' }}
        href="https://goteleport.com/download"
        target="_blank"
      >
        Visit the downloads page
      </ButtonPrimary>
    </Flex>
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
