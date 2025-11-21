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

import React from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import { Box, Card, Flex, H2, H3, Text } from 'design';
import * as Icons from 'design/Icon';
import { P } from 'design/Text/Text';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { useNoMinWidth } from 'teleport/Main';
import { CtaEvent } from 'teleport/services/userEvent';
import useTeleport from 'teleport/useTeleport';

export function SupportContainer({ children }: { children?: React.ReactNode }) {
  const ctx = useTeleport();
  const cluster = ctx.storeUser.state.cluster;

  // showCta returns the premium support value for enterprise customers and true for OSS users
  const showCta = cfg.edition === 'ent' ? !cfg.premiumSupport : true;

  return (
    <Support
      {...cluster}
      isEnterprise={cfg.isEnterprise}
      tunnelPublicAddress={cfg.tunnelPublicAddress}
      isCloud={cfg.isCloud}
      showPremiumSupportCta={showCta}
      authVersion={cluster.authVersion}
    >
      {children}
    </Support>
  );
}

export const Support = ({
  clusterId,
  authVersion,
  publicURL,
  isEnterprise,
  licenseExpiryDateText,
  tunnelPublicAddress,
  isCloud,
  children,
  showPremiumSupportCta,
}: Props) => {
  useNoMinWidth();
  const docs = getDocUrls(authVersion, isEnterprise);

  return (
    <FeatureBox maxWidth="2000px" p={{ _: 2, small: 6 }}>
      <FeatureHeader>
        <FeatureHeaderTitle>Help & Support</FeatureHeaderTitle>
      </FeatureHeader>
      <SupportSectionsWrapper isCloud={isCloud}>
        <SupportSectionCard
          css={`
            grid-column: auto;
            @media screen and (min-width: ${props =>
                props.theme.breakpoints.small}) {
              grid-column: span 2;
            }
          `}
        >
          <Flex
            alignItems={{ _: 'flex-start', small: 'center' }}
            justifyContent="space-between"
            flexDirection={{ _: 'column', small: 'row' }}
            mb={3}
            gap={2}
          >
            <Flex alignItems="center">
              <IconBox>
                <Icons.Question />
              </IconBox>
              <H2>Support and Resource Pages</H2>
            </Flex>
            <SupportButtonBox>
              {showPremiumSupportCta && (
                <ButtonLockedFeature event={CtaEvent.CTA_PREMIUM_SUPPORT}>
                  Unlock Premium Support with&nbsp;Enterprise
                </ButtonLockedFeature>
              )}
            </SupportButtonBox>
          </Flex>
          <SupportLinksFlex>
            <SupportLinkCategory>
              <H3 ml={2} mb={1}>
                Contact Support
              </H3>
              {isEnterprise && !showPremiumSupportCta && (
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
            </SupportLinkCategory>
            <SupportLinkCategory>
              <H3 ml={2} mb={1}>
                Resources
              </H3>
              <ExternalSupportLink
                title="Get Started Guide"
                url={docs.getStarted}
              />
              <ExternalSupportLink title="tsh User Guide" url={docs.tshGuide} />
              <ExternalSupportLink title="Admin Guides" url={docs.adminGuide} />
              <ExternalSupportLink
                title="Troubleshooting Guide"
                url={docs.troubleshooting}
              />
              <DownloadLink isCloud={isCloud} isEnterprise={isEnterprise} />
              <ExternalSupportLink title="FAQ" url={docs.faq} />
            </SupportLinkCategory>
            <SupportLinkCategory>
              <H3 ml={2} mb={1}>
                Updates
              </H3>
              <ExternalSupportLink
                title="Product Changelog"
                url={docs.changeLog}
              />
              <ExternalSupportLink
                title="Teleport Blog"
                url="https://goteleport.com/blog/"
              />
            </SupportLinkCategory>
          </SupportLinksFlex>
        </SupportSectionCard>
        <SupportSectionCard
          css={
            !isCloud &&
            `
            grid-column: span 2;
            @media screen and (max-width: ${props => props.theme.breakpoints.mobile}) {
              grid-column: auto;
            }
          `
          }
        >
          <Flex alignItems="center" justifyContent="start" mb={3}>
            <IconBox>
              <Icons.Cluster />
            </IconBox>
            <H2>Cluster Information</H2>
          </Flex>
          <Flex flexDirection="column" justifyContent="center">
            <P>Cluster Name: {clusterId}</P>
            <P>Teleport Version: {authVersion}</P>
            <P>Public Address: {publicURL}</P>
            {tunnelPublicAddress && (
              <P>Public SSH Tunnel: {tunnelPublicAddress}</P>
            )}
            {isEnterprise && !cfg.isCloud && !!licenseExpiryDateText && (
              <P>License Expiry: {licenseExpiryDateText}</P>
            )}
          </Flex>
        </SupportSectionCard>

        {children}
      </SupportSectionsWrapper>
    </FeatureBox>
  );
};

const SupportSectionsWrapper = styled(Box)<{ isCloud?: boolean }>`
  display: grid;
  gap: ${props => props.theme.space[3]}px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  grid-auto-rows: auto;
  width: 100%;

  @media screen and (max-width: ${props => props.theme.breakpoints.small}) {
    grid-template-columns: 1fr !important;
    gap: ${props => props.theme.space[2]}px;
  }
`;

export const SupportSectionCard = styled(Card)`
  padding: ${props => props.theme.space[4]}px;
  box-shadow: ${props => props.theme.boxShadow[0]};

  @media screen and (max-width: ${props => props.theme.breakpoints.small}) {
    padding: ${props => props.theme.space[3]}px;
  }
`;

export const IconBox = styled(Box)`
  line-height: 0;
  padding: ${props => props.theme.space[2]}px;
  border-radius: ${props => props.theme.radii[3]}px;
  margin-right: ${props => props.theme.space[3]}px;
  background: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  border: ${props => props.theme.borders[1]};
  border-color: ${props => props.theme.colors.interactive.tonal.neutral[2]};

  .icon {
    height: 16px;
    width: 16px;
  }

  @media screen and (max-width: ${props => props.theme.breakpoints.small}) {
    background: transparent;
    margin-right: ${props => props.theme.space[1]}px;
  }
`;

const SupportLinkCategory = styled(Flex)`
  flex-direction: column;
  gap: ${props => props.theme.space[1]}px;
`;

const SupportButtonBox = styled(Box)`
  @media screen and (max-width: ${props => props.theme.breakpoints.small}) {
    width: 100%;
  }
`;

const SupportLinksFlex = styled(Flex)`
  justify-content: space-between;
  flex-wrap: wrap;
  max-width: 70%;
  @media screen and (max-width: ${props => props.theme.breakpoints.medium}) {
    max-width: 100%;
  }
  @media screen and (max-width: ${props => props.theme.breakpoints.small}) {
    flex-direction: column;
    gap: ${props => props.theme.space[3]}px;
    margin-bottom: ${props => props.theme.space[3]}px;
  }
`;

const DataItemFlex = styled(Flex)`
  margin-bottom: ${props => props.theme.space[3]}px;
  @media screen and (max-width: ${props => props.theme.breakpoints.small}) {
    flex-direction: column;
    padding-left: ${props => props.theme.space[2]}px;
  }
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
    adminGuide: withUTM(
      `https://goteleport.com/docs/admin-guides/management/admin/`
    ),
    faq: withUTM(`https://goteleport.com/docs/faq`),
    troubleshooting: withUTM(
      `https://goteleport.com/docs/admin-guides/management/admin/troubleshooting/`
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
        title="Self-Hosting Teleport"
        url="https://goteleport.com/docs/admin-guides/deploy-a-cluster/"
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
  padding: 4px 8px;
  transition: all 0.3s;

  ${props => props.theme.typography.body2}
  &:hover, &:focus {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;

export const DataItem = ({ title = '', data = null }) => (
  <DataItemFlex>
    <Text typography="body2" bold style={{ width: '136px' }}>
      {title}:
    </Text>
    <Text typography="body2">{data}</Text>
  </DataItemFlex>
);

export type Props = {
  clusterId: string;
  authVersion: string;
  publicURL: string;
  licenseExpiryDateText?: string;
  isEnterprise: boolean;
  isCloud: boolean;
  tunnelPublicAddress?: string;
  children?: React.ReactNode;
  showPremiumSupportCta: boolean;
};
