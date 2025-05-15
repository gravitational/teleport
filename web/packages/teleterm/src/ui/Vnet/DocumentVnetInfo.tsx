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

import { useMemo } from 'react';
import styled from 'styled-components';

import { Box, Button, Flex, H1, Stack } from 'design';
import { NewTab } from 'design/Icon';
import { H2, H3, P1, P2 } from 'design/Text/Text';
import { DemoTerminal } from 'shared/components/DemoTerminal';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import Document from 'teleterm/ui/Document';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import type * as docTypes from 'teleterm/ui/services/workspacesService';
import { routing } from 'teleterm/ui/uri';

import appAccessPng from './app-access.png';
import { useVnetAppLauncher } from './useVnetAppLauncher';
import { useVnetContext } from './vnetContext';

export function DocumentVnetInfo(props: {
  visible: boolean;
  doc: docTypes.DocumentVnetInfo;
}) {
  const { doc } = props;
  const { mainProcessClient } = useAppContext();
  const {
    startAttempt,
    stop: stopVnet,
    stopAttempt,
    status,
  } = useVnetContext();
  const { launchVnetWithoutFirstTimeCheck } = useVnetAppLauncher();
  const userAtHost = useMemo(() => {
    const { hostname, username } = mainProcessClient.getRuntimeSettings();
    return `${username}@${hostname}`;
  }, [mainProcessClient]);
  const { rootClusterUri, documentsService } = useWorkspaceContext();
  const proxyHostname = routing.parseClusterName(rootClusterUri);

  const startVnet = async () => {
    await launchVnetWithoutFirstTimeCheck({
      addrToCopy: doc.app?.targetAddress,
      isMultiPort: doc.app?.isMultiPort,
    });
    // Remove targetAddress so that subsequent launches of VNet from this specific doc won't copy
    // the stale app address to the clipboard.
    documentsService.update(doc.uri, { app: undefined });
  };

  return (
    <Document visible={props.visible}>
      <Stack
        gap={6}
        maxWidth="1400px"
        width="100%"
        mx="auto"
        p={{ _: 6, 900: 9, 1200: 11 }}
        // Without this, the Stack would span the whole height of the Document, no matter how much
        // content was displayed in the Stack.
        alignSelf="flex-start"
      >
        <Flex
          width="100%"
          columnGap={7}
          rowGap={4}
          flexWrap="wrap"
          flexDirection={{ _: 'column', 900: 'row' }}
        >
          <Stack gap={3} flex={1}>
            <H1>Teleport VNet</H1>

            <P1>
              VNet automatically proxies connections from your computer
              to&nbsp;TCP apps available through Teleport. Any&nbsp;program on
              your device can connect to an&nbsp;application behind Teleport
              with no&nbsp;extra steps.
            </P1>
            <P1 m={0}>
              Underneath, VNet authenticates the connection with your
              credentials. Everything&nbsp;happens client&#8209;side&nbsp;– VNet
              sets up a&nbsp;local DNS name server, a&nbsp;virtual&nbsp;network
              device, and a&nbsp;proxy.
            </P1>
            <P1 m={0}>VNet makes it easy to connect to…</P1>
          </Stack>

          <Flex
            alignItems="center"
            // Make sure the text in the button doesn't ever break into two lines.
            minWidth="fit-content"
            gap={2}
            flexWrap="wrap"
          >
            <Button
              intent={status.value === 'stopped' ? 'primary' : 'neutral'}
              size="large"
              minWidth="fit-content"
              type="button"
              onClick={status.value === 'stopped' ? startVnet : stopVnet}
              disabled={
                startAttempt.status === 'processing' ||
                stopAttempt.status === 'processing'
              }
            >
              {status.value === 'stopped' ? 'Start VNet' : 'Stop VNet'}
            </Button>
            <Button
              size="large"
              fill="filled"
              intent="neutral"
              as="a"
              href="https://goteleport.com/docs/connect-your-client/vnet/"
              target="_blank"
              minWidth="fit-content"
            >
              Go to Docs
              <NewTab ml={2} />
            </Button>
          </Flex>
        </Flex>

        {/* TCP APIs */}
        <UseCaseSection>
          <TitleAndLearnMoreContainer>
            <H2>TCP APIs</H2>

            <LearnMoreButton href="https://goteleport.com/docs/enroll-resources/application-access/guides/vnet/">
              Learn More
            </LearnMoreButton>
          </TitleAndLearnMoreContainer>

          <ComparisonOption>
            <TextPart>
              <H3>With VNet</H3>
              <P2>Connect directly to the app.</P2>
            </TextPart>

            <DemoTerminal
              flex={demoFlex}
              title={userAtHost}
              text={curlWithVnet(proxyHostname)}
              width="100%"
              boxShadow={2}
            />
          </ComparisonOption>

          <ComparisonOption>
            <TextPart>
              <H3>Without VNet</H3>
              <P2>
                Cannot connect directly, a proxy has to be set up first with
                its&nbsp;own port.
              </P2>
            </TextPart>

            <Stack flex={demoFlex} gap={2} fullWidth>
              <DemoTerminal
                title={userAtHost}
                text={proxyWithoutVnet}
                boxShadow={2}
              />
              <DemoTerminal
                title={userAtHost}
                text={curlWithoutVnet}
                boxShadow={2}
              />
            </Stack>
          </ComparisonOption>
        </UseCaseSection>

        {/* Web apps */}
        <UseCaseSection>
          <TitleAndLearnMoreContainer>
            <H2>Web Applications With 3rd-Party SSO</H2>

            <LearnMoreButton href="https://goteleport.com/docs/enroll-resources/application-access/guides/vnet/#accessing-web-apps-through-vnet">
              Learn More
            </LearnMoreButton>
          </TitleAndLearnMoreContainer>

          <ComparisonOption>
            <Stack gap={4} flex={textFlex}>
              <TextPart flex="initial">
                <H3>With VNet</H3>
                <P2>
                  The app is protected from unauthenticated traffic in a way
                  that is transparent to&nbsp;users, accessible under the same
                  domain with no changes to the SSO setup.
                </P2>
              </TextPart>

              <TextPart flex="initial">
                <H3>Without VNet</H3>
                <P2>
                  The app is not protected from unauthenticated traffic, with
                  access gated only by&nbsp;SSO. If put behind Teleport, the
                  app's domain changes and redirect URLs have to be updated.
                  Users must log&nbsp;in to both Teleport and the SSO provider.
                </P2>
              </TextPart>
            </Stack>

            <DemoPart justifyContent="center">
              <Box
                // Enough width so that the background repeats just twice at full page width.
                width="66%"
                height="100%"
                css={`
                  background: url(${appAccessPng});
                  background-size: contain;
                  background-repeat: space no-repeat;
                  background-position: center;
                `}
              />
            </DemoPart>
          </ComparisonOption>
        </UseCaseSection>
      </Stack>
    </Document>
  );
}

const TitleAndLearnMoreContainer = styled(Flex).attrs({
  gap: 5,
  alignItems: 'center',
  justifyContent: { _: 'space-between', 900: 'flex-start' },
})``;

const LearnMoreButton = styled(Button).attrs({
  fill: 'border',
  intent: 'primary',
  forwardedAs: 'a',
  target: '_blank',
})``;

const UseCaseSection = styled(Stack).attrs({
  gap: 4,
  fullWidth: true,
  as: 'section',
})``;

const ComparisonOption = styled(Flex).attrs({
  gap: 4,
  flexDirection: { _: 'column', 700: 'row' },
})``;

const textFlex = 1;
const TextPart = styled(Stack).attrs(props => ({
  gap: 2,
  flex: textFlex,
  ...props,
}))``;

const demoFlex = 2;
const DemoPart = styled(Flex).attrs({ flex: demoFlex })``;

const curlWithVnet = (
  proxyHostname: string
) => `$ curl http://api.${proxyHostname}
{
  "path": "/",
  "headers": {
    "host": "api.${proxyHostname}",
    "user-agent": "curl/8.7.1",
    "accept": "*/*"
  },
  "method": "GET",
  "body": ""
}
`;

const proxyWithoutVnet = `$ tsh proxy app api --port 61397
Proxying connections to api on 127.0.0.1:61397`;

const curlWithoutVnet = `$ curl http://127.0.0.1:61397
{
  "path": "/",
  "headers": {
    "host": "127.0.0.1:61397",
    "user-agent": "curl/8.7.1",
    "accept": "*/*"
  },
  "method": "GET",
  "body": ""
}
`;
