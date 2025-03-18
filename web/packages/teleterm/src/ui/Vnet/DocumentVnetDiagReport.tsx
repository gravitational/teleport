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

import styled from 'styled-components';

import { Box, Button, Flex, H1, ResponsiveImage, Stack } from 'design';
import { H2, P1 } from 'design/Text/Text';

import Document from 'teleterm/ui/Document';
import type * as docTypes from 'teleterm/ui/services/workspacesService';

import imgNoVnetCurl from './no-vnet-curl.png';
import svgWebAppWithoutVnet from './recording-proxy.svg';
import svgWebAppVnet from './session-recording.svg';
import imgVnetCurl from './vnet-curl.png';

export function DocumentVnetDiagReport(props: {
  visible: boolean;
  doc: docTypes.DocumentVnetDiagReport;
}) {
  return (
    <Document visible={props.visible}>
      <Stack
        gap={5}
        maxWidth="1360px"
        width="100%"
        mx="auto"
        mt={4}
        p={5}
        // Without this, the Stack would span the whole height of the Document, no matter how much
        // content was displayed in the Stack.
        alignSelf="flex-start"
      >
        <Flex width="100%" gap={6} flexWrap="wrap">
          <Stack gap={4}>
            <H1 fontSize={32}>Teleport VNet</H1>

            <P1
              css={`
                max-width: 60ch;
              `}
            >
              VNet automatically proxies connections from your computer to TCP
              apps available through Teleport. Any program on your device can
              connect to an application behind Teleport with no extra steps.
            </P1>
            <P1
              m={0}
              css={`
                max-width: 60ch;
              `}
            >
              Underneath, VNet authenticates the connection with your
              credentials. Everything&nbsp;happens client-side – VNet sets up a
              local DNS name server, a&nbsp;virtual&nbsp;network device, and a
              proxy.
            </P1>
            <P1 m={0}>VNet makes it easy to connect to…</P1>
          </Stack>

          <Flex
            flex={1}
            alignItems="center"
            justifyContent="center"
            // Make sure the text in the button doesn't ever break into two lines.
            minWidth="fit-content"
          >
            <Stack gap={3} alignItems="center">
              <Button size="extra-large">Start VNet</Button>
              <Button
                fill="minimal"
                intent="neutral"
                as="a"
                href="https://goteleport.com/docs/connect-your-client/vnet/"
                target="_blank"
                inputAlignment
              >
                Open Documentation
              </Button>
            </Stack>
          </Flex>
        </Flex>

        <Stack gap={6}>
          {/* TCP APIs */}
          <Stack
            pt={4}
            pb={5}
            px={5}
            gap={4}
            width="100%"
            borderRadius={3}
            backgroundColor="levels.surface"
            alignItems="center"
            css={`
              position: relative;
              // TODO: Create a prop for box shadow.
              box-shadow: ${props => props.theme.boxShadow[0]};
            `}
          >
            <H1>TCP APIs</H1>

            <LearnMoreButton href="https://goteleport.com/docs/enroll-resources/application-access/guides/vnet/">
              Learn More
            </LearnMoreButton>

            <Flex width="100%" gap={8} flexWrap="wrap">
              <Stack flex={1} gap={3} width="100%">
                <Stack alignItems="center" width="100%">
                  <H2>With VNet</H2>
                  {/*
                <P2>No local proxy needed – connect directly to the app.</P2>
                  */}
                  <P1>Connect directly to the app.</P1>
                </Stack>

                <ResponsiveImage
                  src={imgVnetCurl}
                  alt="curl call with VNet"
                  css={`
                    box-shadow: ${props => props.theme.boxShadow[2]};
                  `}
                />
              </Stack>

              <Stack flex={1} gap={3} width="100%">
                <Stack alignItems="center" width="100%">
                  <H2>Without VNet</H2>
                  <P1 textAlign="center">
                    Cannot connect directly, a proxy has to be set up first with
                    its&nbsp;own port.
                  </P1>
                </Stack>

                <ResponsiveImage
                  src={imgNoVnetCurl}
                  alt="curl call without VNet"
                  css={`
                    box-shadow: ${props => props.theme.boxShadow[2]};
                  `}
                />
              </Stack>
            </Flex>
          </Stack>

          {/* Web apps */}
          <Stack
            pt={4}
            pb={5}
            px={5}
            gap={4}
            width="100%"
            borderRadius={3}
            backgroundColor="levels.surface"
            alignItems="center"
            css={`
              position: relative;
              // TODO: Create a prop for box shadow.
              box-shadow: ${props => props.theme.boxShadow[0]};
            `}
          >
            <H1>Web Applications With 3rd-Party SSO</H1>

            <LearnMoreButton href="https://goteleport.com/docs/enroll-resources/application-access/guides/vnet/#accessing-web-apps-through-vnet">
              Learn More
            </LearnMoreButton>
            {/*
              <Button
                fill="minimal"
                intent="neutral"
                as="a"
                href="https://goteleport.com/docs/enroll-resources/application-access/guides/vnet/#accessing-web-apps-through-vnet"
                target="_blank"
                inputAlignment
              >
                Learn More
              </Button>
                */}

            <Flex width="100%" gap={8} flexWrap="wrap">
              <Stack flex={1} gap={3} width="100%">
                <Stack alignItems="center" width="100%">
                  <H2>With VNet</H2>
                  <P1>
                    The app is protected from unauthenticated traffic in a way
                    that is transparent to&nbsp;users, accessible under the same
                    domain with no changes to the SSO setup.
                  </P1>
                </Stack>

                <Box
                  flex={1}
                  backgroundColor="white"
                  px={2}
                  py={3}
                  width="100%"
                  css={`
                    box-shadow: ${props => props.theme.boxShadow[2]};
                  `}
                >
                  <ResponsiveImage
                    alt="Web app with VNet"
                    src={svgWebAppVnet}
                  />
                </Box>
              </Stack>

              <Stack flex={1} gap={3} width="100%">
                <Stack alignItems="center" width="100%">
                  <H2>Without VNet</H2>
                  {/*
                  <P2>
                    Access to the app is gated by both Teleport Proxy Service
                    and 3rd-party SSO. The app is now accessible under the
                    domain of the Proxy Service, so SSO redirect URLs need to be
                    updated.
                  </P2>
                  <P2>
                    Either the app accepts Internet traffic and is protected
                    only by SSO or it is behind Teleport, so admins have to
                    update redirect URLs and users authenticate with both
                    Teleport and SSO.
                  </P2>
                  */}
                  <P1>
                    The app is <em>not</em> protected from unauthenticated
                    traffic, with access gated only by&nbsp;SSO. If put behind
                    Teleport, the app's domain changes and redirect URLs have to
                    be updated. Users must log&nbsp;in to both Teleport and the
                    SSO provider.
                  </P1>
                </Stack>

                <Box
                  flex={1}
                  backgroundColor="white"
                  px={2}
                  py={3}
                  width="100%"
                  css={`
                    box-shadow: ${props => props.theme.boxShadow[2]};
                  `}
                >
                  <ResponsiveImage
                    alt="Web app without VNet"
                    src={svgWebAppWithoutVnet}
                  />
                </Box>
              </Stack>
            </Flex>
          </Stack>
        </Stack>
      </Stack>
    </Document>
  );
}

const LearnMoreButton = styled(Button).attrs({
  size: 'small',
  fill: 'minimal',
  intent: 'neutral',
  forwardedAs: 'a',
  target: '_blank',
})`
  // TODO: Make sure it doesn't overlap with the section header on narrow widths.
  position: absolute;
  right: ${props => props.theme.space[3]}px;
`;
