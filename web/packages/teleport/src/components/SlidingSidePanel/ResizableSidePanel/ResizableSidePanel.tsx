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

import { ButtonIcon, Flex, H2 } from 'design';
import { Cross } from 'design/Icon';

import { SidePanelResizeHandle } from './SidePanelResizeHandle';

export function ResizableSidePanel({
  children,
  onClose,
  onWidthChange,
  title,
  width,
}: {
  children: ReactNode;
  onClose?(): void;
  onWidthChange(width: number): void;
  title?: ReactNode;
  width: number;
}) {
  const showHeader = Boolean(title || onClose);
  return (
    <SidePanel
      data-testid="resizable-side-panel"
      style={{ flexBasis: width, width }}
    >
      <SidePanelResizeHandle panelWidth={width} onWidthChange={onWidthChange} />
      {showHeader && (
        <Header>
          {typeof title === 'string' ? <H2>{title}</H2> : title}
          {onClose && (
            <ButtonIcon aria-label="Close side panel" onClick={onClose}>
              <Cross size="small" />
            </ButtonIcon>
          )}
        </Header>
      )}
      <Content>{children}</Content>
    </SidePanel>
  );
}

const SidePanel = styled(Flex)`
  position: relative;
  overflow: hidden;
  flex: 0 0 auto;
  flex-direction: column;
  min-width: 0;
  border-left: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[1]};
  background: ${p => p.theme.colors.levels.surface};
`;

const Header = styled(Flex)`
  flex: 0 0 auto;
  gap: ${p => p.theme.space[2]}px;
  align-items: center;
  justify-content: space-between;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  border-bottom: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[1]};
`;

const Content = styled(Flex)`
  flex: 1;
  min-height: 0;
  flex-direction: column;
  overflow: auto;
`;
