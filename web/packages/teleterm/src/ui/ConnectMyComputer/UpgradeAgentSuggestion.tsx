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
        upgrading it to {props.proxyVersion} by updating Teleport Connect.{' '}
        <Link href="https://goteleport.com/download" target="_blank">
          Visit the downloads page.
        </Link>
      </Text>
    </Alert>
  );
}
