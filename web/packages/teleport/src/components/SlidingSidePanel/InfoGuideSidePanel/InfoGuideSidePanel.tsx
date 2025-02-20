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

import { PropsWithChildren, useRef, useState } from 'react';

import { Box, ButtonIcon, Flex, Text } from 'design';
import { Cross, Info } from 'design/Icon';

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
      right={0}
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
    p={3}
    css={`
      position: sticky;
      top: 0;
      background: ${p => p.theme.colors.levels.surface};
    `}
  >
    <Flex gap={2}>
      <Info size="small" />
      <Text bold>Info Guide</Text>
    </Flex>
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
    onClick(): void;
  }>
> = ({ onClick, children }) => (
  <Flex alignItems="center">
    {children}
    <ButtonIcon onClick={onClick} data-testid="info-guide-btn-open">
      <Info size="small" />
    </ButtonIcon>
  </Flex>
);
