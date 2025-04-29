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

import React, { PropsWithChildren } from 'react';
import styled from 'styled-components';

import { Box, Button, ButtonIcon, Flex, H3, Link, Text } from 'design';
import { Cross, Info } from 'design/Icon';
import {
  InfoGuideConfig,
  useInfoGuide,
} from 'shared/components/SlidingSidePanel/InfoGuide';

/**
 * Container to render the guide (children) with styled
 * content spacing and header.
 *
 * Meant to be used with SlidingSidePanel.tsx.
 */
export const InfoGuideContainer: React.FC<
  PropsWithChildren<{ onClose(): void; title: React.ReactNode }>
> = ({ onClose, title, children }) => (
  <>
    {children && (
      <Box css={{ height: '100%', overflow: 'auto' }}>
        <InfoGuideHeader title={title} onClose={onClose} />
        <Box px={3} pb={3}>
          {children}
        </Box>
      </Box>
    )}
  </>
);

const InfoGuideHeader = ({
  onClose,
  title: customTitle,
}: {
  onClose(): void;
  title?: React.ReactNode;
}) => {
  let title: React.ReactNode = <Text bold>Page Info</Text>;
  if (customTitle) {
    if (typeof customTitle === 'string') {
      title = <Text bold>{customTitle}</Text>;
    } else {
      title = customTitle;
    }
  }
  return (
    <Flex
      gap={2}
      alignItems="center"
      justifyContent="space-between"
      px={3}
      py={2}
      css={`
        position: sticky;
        top: 0;
        background: ${p => p.theme.colors.levels.surface};
        border-bottom: 1px solid ${p => p.theme.colors.spotBackground[1]};
        z-index: 1;
      `}
    >
      <Text bold>{title}</Text>
      <ButtonIcon onClick={onClose} data-testid="info-guide-btn-close">
        <Cross size="small" />
      </ButtonIcon>
    </Flex>
  );
};

const FilledButtonIcon = styled(Button)`
  width: 32px;
  height: 32px;
  padding: 0;
`;

/**
 * Renders a clickable info icon next to the children.
 */
export const InfoGuideButton: React.FC<
  PropsWithChildren<{
    config: InfoGuideConfig;
    spaceBetween?: boolean;
  }>
> = ({ config, children, spaceBetween = false }) => {
  const { setInfoGuideConfig } = useInfoGuide();

  return (
    <Flex
      alignItems="center"
      gap={2}
      justifyContent={spaceBetween ? 'space-between' : undefined}
    >
      {children}
      <FilledButtonIcon
        intent="neutral"
        onClick={() => setInfoGuideConfig(config)}
        data-testid="info-guide-btn-open"
      >
        <Info size="small" />
      </FilledButtonIcon>
    </Flex>
  );
};

export const InfoTitle = styled(H3)`
  margin-bottom: ${p => p.theme.space[2]}px;
  margin-top: ${p => p.theme.space[3]}px;
`;

export const InfoParagraph = styled(Box)`
  margin-top: ${p => p.theme.space[3]}px;
`;

/**
 * Links used within a paragraph. The color of link is same as main texts
 * so it doesn't take so much focus away from the paragraph.
 */
export const InfoExternalTextLink = styled(Link).attrs({ target: '_blank' })<{
  href: string;
}>`
  color: ${({ theme }) => theme.colors.text.main};
`;

export const InfoUl = styled.ul`
  margin: 0;
  padding-left: ${p => p.theme.space[4]}px;
`;

const InfoLinkLi = styled.li`
  color: ${({ theme }) => theme.colors.interactive.solid.accent.default};
`;

export type ReferenceLink = { title: string; href: string };

export const ReferenceLinks = ({ links }: { links: ReferenceLink[] }) => (
  <>
    <InfoTitle>Reference Links</InfoTitle>
    <InfoUl>
      {links.map(link => (
        <InfoLinkLi key={link.href}>
          <Link target="_blank" href={link.href}>
            {link.title}
          </Link>
        </InfoLinkLi>
      ))}
    </InfoUl>
  </>
);
