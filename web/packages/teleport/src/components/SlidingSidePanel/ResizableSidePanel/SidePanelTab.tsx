/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { type ReactNode } from 'react';
import styled from 'styled-components';

import { Flex, ButtonIcon } from 'design';

export function SidePanelTab({
  ariaLabel,
  children,
  isActive,
  onClick,
}: {
  ariaLabel: string;
  children: ReactNode;
  isActive: boolean;
  onClick(): void;
}) {
  return (
    <Tab
      $isActive={isActive}
      aria-label={ariaLabel}
      aria-pressed={isActive}
      onClick={onClick}
    >
      {children}
    </Tab>
  );
}

export function SidePanelTabs({ children }: { children: ReactNode }) {
  return (
    <TabContainer>
      <StickyTabs>{children}</StickyTabs>
    </TabContainer>
  );
}

const Tab = styled(ButtonIcon)<{ $isActive: boolean }>`
  width: 40px;
  height: 40px;
  border-radius: 6px 0 0 6px;
  background-color: ${p =>
    p.$isActive ? p.theme.colors.interactive.tonal.primary[0] : 'transparent'};
`;

const TabContainer = styled.div`
  position: relative;
  z-index: 2;
  flex: 0 0 0;
  width: 0;
  height: 100%;
`;

const StickyTabs = styled(Flex)`
  position: sticky;
  top: ${p => p.theme.space[3]}px;
  width: 40px;
  flex-direction: column;
  transform: translateX(-100%);
`;
