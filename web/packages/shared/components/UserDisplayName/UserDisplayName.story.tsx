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
import type { ComponentProps, ReactNode } from 'react';
import styled from 'styled-components';

import { Flex, Text } from 'design';

import { UserDisplayName } from './UserDisplayName';

const meta = {
  title: 'Shared/UserDisplayName',
  component: Wrapper,
  args: {
    username: 'alice@example.com',
    primaryText: 'Alice Jones',
    secondaryText: 'Engineering',
    layout: 'tooltip',
  },
  argTypes: {
    layout: {
      control: { type: 'radio' },
      options: ['inline', 'stacked', 'tooltip'],
    },
  },
  decorators: [
    Story => (
      <Flex maxWidth="320px">
        <Story />
      </Flex>
    ),
  ],
} satisfies Meta<typeof Wrapper>;

export default meta;

type Story = StoryObj<typeof meta>;

function Wrapper(props: ComponentProps<typeof UserDisplayName>) {
  return <UserDisplayName {...props} />;
}

export const Playground: Story = {};

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
        description="Username and secondary render inside the parenthetical group."
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
        description="Username and secondary share the supporting line."
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
        description="Username renders inside the parenthetical group."
      >
        <UserDisplayName
          username="alice@example.com"
          primaryText="Alice Jones"
          layout="inline"
        />
      </LayoutExample>
      <LayoutExample
        value="stacked"
        description="Username renders on the supporting line."
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

export const LayoutVariantsWithoutPrimary: Story = {
  render: () => (
    <Flex
      alignItems="flex-start"
      flexDirection="column"
      gap={3}
      maxWidth="320px"
    >
      <LayoutExample
        value="inline"
        description="Username and secondary stay on one line."
      >
        <UserDisplayName
          username="alice@example.com"
          secondaryText="Engineering"
          layout="inline"
        />
      </LayoutExample>
      <LayoutExample
        value="stacked"
        description="Username stays first, with secondary below it."
      >
        <UserDisplayName
          username="alice@example.com"
          secondaryText="Engineering"
          layout="stacked"
        />
      </LayoutExample>
      <LayoutExample
        value="tooltip"
        description="Username stays visible, with secondary below it."
      >
        <UserDisplayName
          username="alice@example.com"
          secondaryText="Engineering"
          layout="tooltip"
        />
      </LayoutExample>
    </Flex>
  ),
};

export const LayoutVariantsWithoutPrimaryOrSecondary: Story = {
  render: () => (
    <Flex
      alignItems="flex-start"
      flexDirection="column"
      gap={3}
      maxWidth="320px"
    >
      <LayoutExample value="inline" description="Username is the only value.">
        <UserDisplayName username="alice@example.com" layout="inline" />
      </LayoutExample>
      <LayoutExample value="stacked" description="Username is the only value.">
        <UserDisplayName username="alice@example.com" layout="stacked" />
      </LayoutExample>
      <LayoutExample value="tooltip" description="Username is the only value.">
        <UserDisplayName username="alice@example.com" layout="tooltip" />
      </LayoutExample>
    </Flex>
  ),
};

// LongValues demonstrates that overly long values are truncated with an
// ellipsis within their container instead of pushing the layout outward.
export const LongValues: Story = {
  render: () => (
    <Flex alignItems="stretch" flexDirection="column" gap={3} width="240px">
      <LayoutExample
        value="inline"
        description="Grouped values are truncated within a narrow container."
      >
        <UserDisplayName
          username="alice.jones.engineering@very-long-example-domain.com"
          primaryText="Alice Jones-Anderson-Engineering"
          secondaryText="Platform Engineering"
          layout="inline"
        />
      </LayoutExample>
      <LayoutExample
        value="stacked"
        description="Each stacked value line truncates independently."
      >
        <UserDisplayName
          username="alice.jones.engineering@very-long-example-domain.com"
          primaryText="Alice Jones-Anderson-Engineering"
          secondaryText="Platform Engineering"
          layout="stacked"
        />
      </LayoutExample>
      <LayoutExample
        value="tooltip"
        description="Primary truncates inline; full username appears on hover."
      >
        <UserDisplayName
          username="alice.jones.engineering@very-long-example-domain.com"
          primaryText="Alice Jones-Anderson-Engineering"
          secondaryText="Platform Engineering"
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
  box-sizing: border-box;
  max-width: 100%;
  padding: ${props => props.theme.space[1]}px ${props => props.theme.space[2]}px;
  border-left: 2px solid
    ${props => props.theme.colors.interactive.solid.primary.default};
  border-radius: 0 4px 4px 0;
  background: ${props => props.theme.colors.levels.surface};
`;
