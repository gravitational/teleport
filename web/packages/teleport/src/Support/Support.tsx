/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Link } from 'react-router-dom';
import { Box, Card, Flex, Text } from 'design';
import * as Icons from 'design/Icon';

import styled from 'styled-components';

import { FeatureBox } from 'teleport/components/Layout';
import useTeleport from 'teleport/useTeleport';
import cfg from 'teleport/config';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { CtaEvent } from 'teleport/services/userEvent';

export default function Container({
  children,
}: {
  children?: React.ReactNode;
}) {
  const ctx = useTeleport();
  const cluster = ctx.storeUser.state.cluster;

  // showCTA returns the premium support value for enterprise customers and true for OSS users
  const showCTA = cfg.isEnterprise ? ctx.lockedFeatures.premiumSupport : true;

  return (
    <Support
      {...cluster}
      isEnterprise={cfg.isEnterprise}
      tunnelPublicAddress={cfg.tunnelPublicAddress}
      isCloud={cfg.isCloud}
      showPremiumSupportCTA={showCTA}
      children={children}
    />
  );
}

export const Support = ({
  clusterId,
  authVersion,
  publicURL,
  isEnterprise,
  tunnelPublicAddress,
  isCloud,
  children,
  showPremiumSupportCTA,
}: Props) => {
  const docs = getDocUrls(authVersion, isEnterprise);

  return (
    <FeatureBox pt="4">
      <Card px={5} pt={1} pb={6}>
        <Flex justifyContent="space-between" flexWrap="wrap">
          <Box>
            <Header title="Support" icon={<Icons.Headset />} />
            {isEnterprise && !showPremiumSupportCTA && (
              <ExternalSupportLink
                title="Create a Support Ticket"
                url="https://support.goteleport.com"
              />
            )}
            <ExternalSupportLink
              title="Ask the Community Questions"
              url="https://github.com/gravitational/teleport/discussions"
            />
            <ExternalSupportLink
              title="Request a New Feature"
              url="https://github.com/gravitational/teleport/issues/new/choose"
            />
            <ExternalSupportLink
              title="Send Product Feedback"
              url="mailto:support@goteleport.com"
            />
            {showPremiumSupportCTA && (
              <ButtonLockedFeature event={CtaEvent.CTA_PREMIUM_SUPPORT}>
                Unlock Premium Support w/Enterprise
              </ButtonLockedFeature>
            )}
          </Box>
          <Box>
            <Header title="Resources" icon={<Icons.BookOpenText />} />
            <ExternalSupportLink title="Get Started" url={docs.getStarted} />
            <ExternalSupportLink title="tsh User Guide" url={docs.tshGuide} />
            <ExternalSupportLink title="Admin Guides" url={docs.adminGuide} />
            <DownloadLink isCloud={isCloud} isEnterprise={isEnterprise} />
            <ExternalSupportLink title="FAQ" url={docs.faq} />
          </Box>
          <Box>
            <Header title="Troubleshooting" icon={<Icons.Graph />} />
            <ExternalSupportLink
              title="Monitoring & Debugging"
              url={docs.troubleshooting}
            />
          </Box>
          <Box>
            <Header title="Updates" icon={<Icons.NotificationsActive />} />
            <ExternalSupportLink
              title="Product Changelog"
              url={docs.changeLog}
            />
            <ExternalSupportLink
              title="Teleport Blog"
              url="https://goteleport.com/blog/"
            />
          </Box>
        </Flex>
      </Card>
      <DataContainer title="Cluster Information">
        <DataItem title="Cluster Name" data={clusterId} />
        <DataItem title="Teleport Version" data={authVersion} />
        <DataItem title="Public Address" data={publicURL} />
        {tunnelPublicAddress && (
          <DataItem title="Public SSH Tunnel" data={tunnelPublicAddress} />
        )}
      </DataContainer>

      {children}
    </FeatureBox>
  );
};

export const DataContainer: React.FC<{ title: string }> = ({
  title,
  children,
}) => (
  <StyledDataContainer mt={4} borderRadius={3} px={5} py={4}>
    <Text as="h5" mb={4} fontWeight="bold" caps>
      {title}
    </Text>
    {children}
  </StyledDataContainer>
);

const StyledDataContainer = styled(Box)`
  border: 1px solid ${props => props.theme.colors.spotBackground[1]};
`;

/**
 * getDocUrls returns an object of URL's appended with
 * UTM, version, and type of teleport.
 *
 * @param version teleport version retrieved from cluster info.
 */
const getDocUrls = (version = '', isEnterprise: boolean) => {
  const verPrefix = isEnterprise ? 'e' : 'oss';

  /**
   * withUTM appends URL with UTM parameters.
   * anchor hashes must be appended at end of URL otherwise it is ignored.
   *
   * @param url the full link to the specific documentation.
   * @param anchorHash the hash in URL that predefines scroll location in the page.
   */
  const withUTM = (url = '', anchorHash = '') =>
    `${url}?product=teleport&version=${verPrefix}_${version}${anchorHash}`;

  return {
    getStarted: withUTM(`https://goteleport.com/docs/get-started/`),
    tshGuide: withUTM(`https://goteleport.com/docs/connect-your-client/tsh/`),
    adminGuide: withUTM(`https://goteleport.com/docs/management/admin/`),
    faq: withUTM(`https://goteleport.com/docs/faq`),
    troubleshooting: withUTM(
      `https://goteleport.com/docs/management/admin/troubleshooting/`
    ),

    // there isn't a version-specific changelog page
    changeLog: withUTM('https://goteleport.com/docs/changelog'),
  };
};

const DownloadLink = ({
  isCloud,
  isEnterprise,
}: {
  isCloud: boolean;
  isEnterprise: boolean;
}) => {
  if (isCloud) {
    return (
      <StyledSupportLink as={Link} to={cfg.routes.downloadCenter}>
        Download Page
      </StyledSupportLink>
    );
  }

  if (isEnterprise) {
    return (
      <ExternalSupportLink
        title="Download Page"
        url="https://goteleport.com/docs/choose-an-edition/teleport-enterprise/introduction/?scope=enterprise#dedicated-account-dashboard"
      />
    );
  }

  return (
    <ExternalSupportLink
      title="Download Page"
      url="https://goteleport.com/download/"
    />
  );
};

const ExternalSupportLink = ({ title = '', url = '' }) => (
  <StyledSupportLink href={url} target="_blank">
    {title}
  </StyledSupportLink>
);

const StyledSupportLink = styled.a.attrs({
  rel: 'noreferrer',
})`
  display: block;
  color: ${props => props.theme.colors.text.main};
  border-radius: 4px;
  text-decoration: none;
  margin-bottom: 8px;
  padding: 4px 8px;
  transition: all 0.3s;

  ${props => props.theme.typography.body2}
  &:hover, &:focus {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;

const StyledHeader = styled(Flex)`
  border-bottom: 1px solid ${props => props.theme.colors.spotBackground[2]};
`;

export const DataItem = ({ title = '', data = null }) => (
  <Flex mb={3}>
    <Text typography="body2" bold style={{ width: '130px' }}>
      {title}:
    </Text>
    <Text typography="body2">{data}</Text>
  </Flex>
);

const Header = ({ title = '', icon = null }) => (
  <StyledHeader alignItems="center" mb={3} width={210} mt={4} pb={2}>
    {icon}
    <Text as="h5" ml={2} caps>
      {title}
    </Text>
  </StyledHeader>
);

export type Props = {
  clusterId: string;
  authVersion: string;
  publicURL: string;
  isEnterprise: boolean;
  isCloud: boolean;
  tunnelPublicAddress?: string;
  children?: React.ReactNode;
  showPremiumSupportCTA: boolean;
};
