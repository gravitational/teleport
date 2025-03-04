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

import { Box, ButtonIcon, Flex, Text } from 'design';
import { Cross, Info } from 'design/Icon';

import { useInfoGuide } from 'teleport/Main/InfoGuideContext';
import { zIndexMap } from 'teleport/Navigation/zIndexMap';

import { SlidingSidePanel } from '..';

export const infoGuidePanelWidth = 300;

/**
 * An info panel that always slides from the right and supports closing
 * from inside of panel (by clicking on x button from the sticky header).
 */
export const InfoGuideSidePanel: React.FC<
  PropsWithChildren<{
    isVisible: boolean;
    onClose(): void;
  }>
> = ({ isVisible, children, onClose }) => {
  return (
    <SlidingSidePanel
      isVisible={isVisible}
      skipAnimation={false}
      panelWidth={infoGuidePanelWidth}
      zIndex={zIndexMap.infoGuideSidePanel}
      slideFrom="right"
    >
      <Box css={{ height: '100%', overflow: 'auto' }}>
        <InfoGuideHeader onClose={onClose} />
        <Box px={3} pb={3}>
          {children}
        </Box>
      </Box>
    </SlidingSidePanel>
  );
};

const InfoGuideHeader = ({ onClose }: { onClose(): void }) => (
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
    <Text bold>Info Guide</Text>
    <ButtonIcon onClick={onClose} data-testid="info-guide-btn-close">
      <Cross size="small" />
    </ButtonIcon>
  </Flex>
);

/**
 * Renders a clickable info icon next to the children.
 */
export const InfoGuideWrapper: React.FC<
  PropsWithChildren<{
    guide: JSX.Element;
  }>
> = ({ guide, children }) => {
  const { setInfoGuideElement } = useInfoGuide();

  return (
    <Flex alignItems="center" gap={2}>
      {children}
      <ButtonIcon
        onClick={() => setInfoGuideElement(guide)}
        data-testid="info-guide-btn-open"
      >
        <Info size="small" />
      </ButtonIcon>
    </Flex>
  );
};
