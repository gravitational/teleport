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

import { InfoGuideConfig, useInfoGuide } from 'teleport/Main/InfoGuideContext';
import { zIndexMap } from 'teleport/Navigation/zIndexMap';

import { SlidingSidePanel } from '..';

export const infoGuidePanelWidth = 300;

/**
 * An info panel that always slides from the right and supports closing
 * from inside of panel (by clicking on x button from the sticky header).
 */
export const InfoGuideSidePanel = () => {
  const { infoGuideConfig, setInfoGuideConfig } = useInfoGuide();
  const infoGuideSidePanelOpened = infoGuideConfig != null;

  return (
    <SlidingSidePanel
      isVisible={infoGuideSidePanelOpened}
      skipAnimation={false}
      panelWidth={infoGuideConfig?.panelWidth || infoGuidePanelWidth}
      zIndex={zIndexMap.infoGuideSidePanel}
      slideFrom="right"
    >
      <Box css={{ height: '100%', overflow: 'auto' }}>
        <InfoGuideHeader
          title={infoGuideConfig?.title}
          onClose={() => setInfoGuideConfig(null)}
        />
        <Box px={3} pb={3}>
          {infoGuideConfig?.guide}
        </Box>
      </Box>
    </SlidingSidePanel>
  );
};

const InfoGuideHeader = ({
  onClose,
  title = 'Page Info',
}: {
  onClose(): void;
  title?: string;
}) => (
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
    `}
  >
    <Text bold>{title}</Text>
    <ButtonIcon onClick={onClose} data-testid="info-guide-btn-close">
      <Cross size="small" />
    </ButtonIcon>
  </Flex>
);

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
