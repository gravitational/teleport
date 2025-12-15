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

import { PropsWithChildren, ReactNode } from 'react';
import styled from 'styled-components';

import { ButtonText } from 'design/Button/Button';
import Flex from 'design/Flex/Flex';
import Text from 'design/Text/Text';

export function Panel(
  props: PropsWithChildren & {
    title: string;
    isSubPanel?: boolean;
    action?: {
      label: string;
      onClick: () => void;
      iconLeft?: ReactNode;
      iconRight?: ReactNode;
      disabled?: boolean;
    };
    testId?: string;
  }
) {
  const { title, isSubPanel = false, action, children, testId } = props;
  return (
    <section>
      <Container data-testid={testId}>
        <TitleContainer>
          <PanelTitleText $isSubPanel={isSubPanel}>{title}</PanelTitleText>
          {action ? (
            <ActionButton onClick={action.onClick} disabled={action.disabled}>
              {action.iconLeft}
              {action.label}
              {action.iconRight}
            </ActionButton>
          ) : undefined}
        </TitleContainer>
        {children}
      </Container>
    </section>
  );
}

const Container = styled(Flex)`
  flex-direction: column;
`;

const TitleContainer = styled(Flex)`
  align-items: center;
  justify-content: space-between;
  padding: ${p => p.theme.space[3]}px;
  gap: 8px;
`;

export const PanelTitleText = styled(Text).attrs<{ $isSubPanel?: boolean }>(
  ({ $isSubPanel = false }) => ({
    as: $isSubPanel ? 'h3' : 'h2',
    typography: $isSubPanel ? 'h3' : 'h2',
  })
)``;

const ActionButton = styled(ButtonText)`
  padding-left: 8px;
  padding-right: 8px;
  gap: 8px;
`;
