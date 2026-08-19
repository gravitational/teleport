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

import {
  ButtonLink,
  Card,
  Flex,
  GraphIcon,
  H1,
  H2,
  H3,
  Subtitle2,
  QuestionIcon,
  SimpleGrid,
  Stack,
  styled,
} from '@gravitational/design-system';
import React from 'react';
import { Link } from 'react-router';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
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
    <SimpleGrid
      columns={{ base: 1, sm: 2 }}
      gap={{ base: 2, sm: 4 }}
      width="100%"
      maxWidth="2000px"
      minHeight="100%"
      flexShrink={0}
      alignContent="start"
      p={{ base: 2, sm: 7 }}
      pb={{ base: 6, sm: 10 }}
    >
      <H1
        gridColumn="1 / -1"
        whiteSpace="nowrap"
        py={3}
        mb={{ base: 4, sm: 2 }}
      >
        Help & Support
      </H1>
      <SupportSectionCard
        title="Support and Resource Pages"
        icon={<QuestionIcon boxSize={4} />}
        titleAction={
          showPremiumSupportCta && (
            <ButtonLockedFeature
              event={CtaEvent.CTA_PREMIUM_SUPPORT}
              width={{ _: '100%', small: 'auto' }}
            >
              Unlock Premium Support with&nbsp;Enterprise
            </ButtonLockedFeature>
          )
        }
        display="block"
        gridColumn="1 / -1"
      >
        <Flex
          justify="space-between"
          wrap="wrap"
          direction={{ base: 'column', sm: 'row' }}
          maxWidth={{ md: '70%' }}
          gap={{ base: 4, sm: 0 }}
          mb={{ base: 4, sm: 0 }}
        >
          <SupportLinkCategory title="Contact Support">
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
          <SupportLinkCategory title="Resources">
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
          <SupportLinkCategory title="Updates">
            <ExternalSupportLink
              title="Product Changelog"
              url={docs.changeLog}
            />
            <ExternalSupportLink
              title="Upcoming Releases"
              url={docs.upcomingReleases}
            />
            <ExternalSupportLink
              title="Teleport Blog"
              url="https://goteleport.com/blog/"
            />
          </SupportLinkCategory>
        </Flex>
      </SupportSectionCard>
      <SupportSectionCard
        title="Cluster Information"
        icon={<GraphIcon boxSize={4} />}
        gridColumn={isCloud ? undefined : '1 / -1'}
      >
        <Stack gap={4}>
          <Subtitle2>Cluster Name: {clusterId}</Subtitle2>
          <Subtitle2>Teleport Version: {authVersion}</Subtitle2>
          <Subtitle2>Public Address: {publicURL}</Subtitle2>
          {tunnelPublicAddress && (
            <Subtitle2>Public SSH Tunnel: {tunnelPublicAddress}</Subtitle2>
          )}
          {isEnterprise && !cfg.isCloud && !!licenseExpiryDateText && (
            <Subtitle2>License Expiry: {licenseExpiryDateText}</Subtitle2>
          )}
        </Stack>
      </SupportSectionCard>

      {children}
    </SimpleGrid>
  );
};

type SupportSectionCardProps = Omit<Card.RootProps, 'css' | 'title'> & {
  title: string;
  icon: React.ReactNode;
  titleAction?: React.ReactNode;
};

export const SupportSectionCard = ({
  title,
  icon,
  titleAction,
  children,
  ...cardProps
}: SupportSectionCardProps) => {
  const heading = (
    <H2 display="flex" alignItems="center" mb={titleAction ? 0 : 4}>
      <IconBox>{icon}</IconBox>
      {title}
    </H2>
  );

  return (
    <Card.Root boxShadow="xs" p={{ base: 4, sm: 5 }} {...cardProps}>
      {titleAction ? (
        <Flex align="center" justify="space-between" wrap="wrap" gap={2} mb={4}>
          {heading}
          {titleAction}
        </Flex>
      ) : (
        heading
      )}
      {children}
    </Card.Root>
  );
};

const IconBox = styled('span', {
  base: {
    lineHeight: 0,
    padding: 2,
    borderRadius: 'md',
    marginRight: { base: 1, sm: 4 },
    border: 'sm',
    borderColor: 'interactive.tonal.neutral.2',
    background: { base: 'transparent', sm: 'interactive.tonal.neutral.0' },

    '& .icon': {
      height: '16px',
      width: '16px',
    },
  },
});

const SupportLinkCategory = ({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) => (
  <Stack gap={1}>
    <H3 ml={2} mb={1}>
      {title}
    </H3>
    {children}
  </Stack>
);

/**
 * getDocUrls returns an object of URLs appended with
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

    // these pages aren't verison-specific
    changeLog: withUTM('https://goteleport.com/docs/changelog'),
    upcomingReleases: withUTM('https://goteleport.com/docs/upcoming-releases'),
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
      <SupportLink>
        <Link to={cfg.routes.downloadCenter} rel="noreferrer">
          Download Page
        </Link>
      </SupportLink>
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
  <SupportLink>
    <a href={url} target="_blank" rel="noreferrer">
      {title}
    </a>
  </SupportLink>
);

const SupportLink = ({ children }: { children: React.ReactElement }) => (
  <ButtonLink
    asChild
    display="block"
    textStyle="body2"
    fontWeight="light"
    lineHeight="24px"
    color="text.main"
    textDecoration="none"
    whiteSpace="normal"
    borderWidth={0}
    minW="auto"
    minH="auto"
    py={1}
    _hover={{
      background: 'interactive.tonal.neutral.0',
      color: 'text.main',
      textDecoration: 'none',
    }}
    _active={{
      background: 'interactive.tonal.neutral.0',
      color: 'text.main',
    }}
    _focusVisible={{
      background: 'interactive.tonal.neutral.0',
      color: 'text.main',
      borderColor: 'transparent',
    }}
  >
    {children}
  </ButtonLink>
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
