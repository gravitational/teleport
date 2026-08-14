/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useEffect } from 'react';
import { useLocation, useParams } from 'react-router';
import styled from 'styled-components';

import { Box, ButtonPrimary, Link, Stack, Text } from 'design';
import { getPlatform } from 'design/platform';
import { makeDeepLinkWithSafeInput } from 'shared/deepLinks';
import { processRedirectUri } from 'shared/redirects';

import {
  DownloadConnect,
  DownloadLink,
  getConnectDownloadLinks,
} from 'teleport/components/DownloadConnect/DownloadConnect';
import cfg from 'teleport/config';
import { useNoMinWidth } from 'teleport/Main';
import history from 'teleport/services/history/history';
import useTeleport from 'teleport/useTeleport';

export const PassthroughPage = () => {
  const ctx = useTeleport();
  const { search } = useLocation();
  const { id = '', token = '' } = useParams<{
    id: string;
    token: string;
  }>();
  const redirect_uri = new URLSearchParams(search).get('redirect_uri');

  const { cluster, username } = ctx.storeUser.state;
  const deviceTrustAuthorize = makeDeepLinkWithSafeInput({
    proxyHost: cluster?.publicURL,
    username: username,
    path: '/authenticate_web_device',
    searchParams: {
      id,
      token,
      redirect_uri,
    },
  });
  const platform = getPlatform();
  const downloadLinks = getConnectDownloadLinks(platform, cluster.proxyVersion);

  useEffect(() => {
    // Use _self as the target to avoid opening a new blank tab to launch the deep link.
    // On Chrome the blank tab would disappear after approving a deep link launch, but in Firefox
    // and Safari it'd stay open. With _self, even if the user cancels the launch there will be no
    // tab to close.
    window.open(deviceTrustAuthorize, '_self');

    // the deviceWebToken is only valid for 5 minutes, so we can forward
    // to the dashboard
    const id = window.setTimeout(
      () => {
        history.push(cfg.routes.root, true);
      },
      1000 * 60 * 5 /* 5 minutes */
    );

    return () => window.clearTimeout(id);
  }, [deviceTrustAuthorize]);

  return (
    <DeviceTrustConnectPassthrough
      redirectUri={redirect_uri}
      downloadLinks={downloadLinks}
      authorizeWebDeviceDeepLink={deviceTrustAuthorize}
    />
  );
};

export const DeviceTrustConnectPassthrough = ({
  authorizeWebDeviceDeepLink,
  redirectUri,
  downloadLinks,
}: {
  authorizeWebDeviceDeepLink: string;
  redirectUri?: string;
  downloadLinks: Array<DownloadLink>;
}) => {
  // Gets rid of the horizontal scrollbar on smaller screens.
  useNoMinWidth();

  return (
    <Wrapper>
      <Text fontSize={7} mb={5}>
        Click <BoldText>Open Teleport Connect</BoldText> on the dialog shown by
        your browser.
      </Text>

      <Text fontSize={7} mb={4}>
        If you don't see a dialog, click{' '}
        <BoldText>Launch Teleport Connect</BoldText> below.
      </Text>

      <Box mb={10}>
        <ButtonPrimary
          size="extra-large"
          textTransform="none"
          as="a"
          href={authorizeWebDeviceDeepLink}
        >
          Launch Teleport Connect
        </ButtonPrimary>
      </Box>

      <Stack alignItems="center">
        <Text fontSize={3}>Don't have Teleport Connect?</Text>
        <DownloadConnect downloadLinks={downloadLinks} />
      </Stack>

      <SkipAuthNotice>
        <Text>
          You can{' '}
          <Link href={processRedirectUri(redirectUri)}>
            continue without Device Trust
          </Link>{' '}
          but you will not be able to connect to resources that require
          Device&nbsp;Trust.
        </Text>
      </SkipAuthNotice>
    </Wrapper>
  );
};

const SkipAuthNotice = styled(Box).attrs({
  textAlign: 'center',
  marginTop: 'auto',
})``;

const BoldText = styled(Text).attrs({ as: 'span', bold: true })``;

const Wrapper = styled(Stack).attrs({
  fullWidth: true,
  height: '100%',
  textAlign: 'center',
  lineHeight: '32px',
  pt: '5vh',
  px: 3,
  pb: 3,
})``;
