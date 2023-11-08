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

import { ButtonSecondary } from 'design/Button';
import { Platform, getPlatform } from 'design/platform';
import { Text, Flex } from 'design';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { Path, makeDeepLinkWithSafeInput } from 'shared/deepLinks';

import useTeleport from 'teleport/useTeleport';

import { ActionButtons, Header, StyledBox } from 'teleport/Discover/Shared';

import type { AgentStepProps } from '../../types';

export function SetupConnect(props: AgentStepProps) {
  const ctx = useTeleport();
  const { cluster, username } = ctx.storeUser.state;
  const platform = getPlatform();
  const downloadLinks = getConnectDownloadLinks(platform, cluster.proxyVersion);
  const connectMyComputerDeepLink = makeDeepLinkWithSafeInput({
    proxyHost: cluster.publicURL,
    username,
    path: Path.ConnectMyComputer,
  });

  return (
    <Flex flexDirection="column" alignItems="flex-start" mb={2} gap={4}>
      <Header>Set Up Teleport Connect</Header>

      <StyledBox>
        <Text bold>Step 1: Download and Install Teleport Connect</Text>

        <Text typography="subtitle1" mb={2}>
          Teleport Connect is a native desktop application for browsing and
          accessing your resources. It can also connect your computer as an SSH
          resource and scope access to a unique role so it is not automatically
          shared with anyone else in the cluster.
          <br />
          <br />
          Once you’ve downloaded Teleport Connect, run the installer to add it
          to your computer’s applications.
        </Text>

        <Flex flexWrap="wrap" alignItems="baseline" gap={2}>
          <DownloadConnect downloadLinks={downloadLinks} />
          <Text typography="subtitle1">
            Already have Teleport Connect? Skip to the next step.
          </Text>
        </Flex>
      </StyledBox>

      <StyledBox>
        <Text bold>Step 2: Sign In and Connect My Computer</Text>

        <Text typography="subtitle1" mb={2}>
          The button below will open Teleport Connect and once you are logged
          in, it will prompt you to connect your computer. From there, follow
          the instructions in Teleport Connect, and this page will update when
          your computer is detected in the cluster.
        </Text>

        <ButtonSecondary as="a" href={connectMyComputerDeepLink}>
          Sign In & Connect My Computer
        </ButtonSecondary>
      </StyledBox>

      <ActionButtons
        onProceed={() => {}}
        disableProceed={true}
        onPrev={props.prevStep}
      />
    </Flex>
  );
}

type DownloadLink = { text: string; url: string };

const DownloadConnect = (props: { downloadLinks: Array<DownloadLink> }) => {
  if (props.downloadLinks.length === 1) {
    const downloadLink = props.downloadLinks[0];
    return (
      <ButtonSecondary as="a" href={downloadLink.url}>
        Download Teleport Connect
      </ButtonSecondary>
    );
  }

  return (
    <MenuButton buttonText="Download Teleport Connect">
      {props.downloadLinks.map(link => (
        <MenuItem key={link.url} as="a" href={link.url}>
          {link.text}
        </MenuItem>
      ))}
    </MenuButton>
  );
};

function getConnectDownloadLinks(
  platform: Platform,
  proxyVersion: string
): Array<DownloadLink> {
  switch (platform) {
    case Platform.Windows:
      return [
        {
          text: 'Teleport Connect',
          url: `https://cdn.teleport.dev/Teleport Connect Setup-${proxyVersion}.exe`,
        },
      ];
    case Platform.macOS:
      return [
        {
          text: 'Teleport Connect',
          url: `https://cdn.teleport.dev/Teleport Connect-${proxyVersion}.dmg`,
        },
      ];
    case Platform.Linux:
      return [
        {
          text: 'DEB',
          url: `https://cdn.teleport.dev/teleport-connect_${proxyVersion}_amd64.deb`,
        },
        {
          text: 'RPM',
          url: `https://cdn.teleport.dev/teleport-connect-${proxyVersion}.x86_64.rpm`,
        },

        {
          text: 'tar.gz',
          url: `https://cdn.teleport.dev/teleport-connect-${proxyVersion}-x64.tar.gz`,
        },
      ];
  }
}
