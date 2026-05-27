/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import type { Meta, StoryObj } from '@storybook/react-vite';
import type { ReactNode } from 'react';
import styled from 'styled-components';

import { Flex, Text } from 'design';

import { UserDisplayName } from './UserDisplayName';

const meta: Meta<typeof UserDisplayName> = {
  title: 'Shared/UserDisplayName',
  component: UserDisplayName,
  args: {
    username: 'alice@example.com',
    primaryText: 'Alice Jones',
    secondaryText: 'Engineering',
  },
};

export default meta;

type Story = StoryObj<typeof UserDisplayName>;

export const LayoutVariants: Story = {
  render: () => (
    <Flex
      alignItems="flex-start"
      flexDirection="column"
      gap={3}
      maxWidth="320px"
    >
      <LayoutExample
        value="inline"
        description="Primary, secondary, and username stay on one line."
      >
        <UserDisplayName
          username="alice@example.com"
          primaryText="Alice Jones"
          secondaryText="Engineering"
          layout="inline"
        />
      </LayoutExample>
      <LayoutExample
        value="stacked"
        description="Primary stays first, with supporting values below it."
      >
        <UserDisplayName
          username="alice@example.com"
          primaryText="Alice Jones"
          secondaryText="Engineering"
          layout="stacked"
        />
      </LayoutExample>
      <LayoutExample
        value="tooltip"
        description="Primary stays visible, and username appears in a tooltip."
      >
        <UserDisplayName
          username="alice@example.com"
          primaryText="Alice Jones"
          secondaryText="Engineering"
          layout="tooltip"
        />
      </LayoutExample>
    </Flex>
  ),
};

export const LayoutVariantsWithoutSecondary: Story = {
  render: () => (
    <Flex
      alignItems="flex-start"
      flexDirection="column"
      gap={3}
      maxWidth="320px"
    >
      <LayoutExample
        value="inline"
        description="Primary and username stay on one line."
      >
        <UserDisplayName
          username="alice@example.com"
          primaryText="Alice Jones"
          layout="inline"
        />
      </LayoutExample>
      <LayoutExample
        value="stacked"
        description="Primary stays first, with username below it."
      >
        <UserDisplayName
          username="alice@example.com"
          primaryText="Alice Jones"
          layout="stacked"
        />
      </LayoutExample>
      <LayoutExample
        value="tooltip"
        description="Primary stays visible, and username appears in a tooltip."
      >
        <UserDisplayName
          username="alice@example.com"
          primaryText="Alice Jones"
          layout="tooltip"
        />
      </LayoutExample>
    </Flex>
  ),
};

function LayoutExample({
  children,
  description,
  value,
}: {
  children: ReactNode;
  description: string;
  value: string;
}) {
  return (
    <Flex alignItems="flex-start" flexDirection="column" gap={2}>
      <StoryNote>
        <Text typography="body3">{`layout="${value}"`}</Text>
        <Text typography="body3" color="text.muted">
          {description}
        </Text>
      </StoryNote>
      {children}
    </Flex>
  );
}

const StoryNote = styled.div`
  padding: ${props => props.theme.space[1]}px ${props => props.theme.space[2]}px;
  border-left: 2px solid
    ${props => props.theme.colors.interactive.solid.primary.default};
  border-radius: 0 4px 4px 0;
  background: ${props => props.theme.colors.levels.surface};
`;
