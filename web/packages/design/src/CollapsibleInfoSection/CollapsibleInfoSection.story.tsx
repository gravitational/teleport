/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { action } from '@storybook/addon-actions';
import { Meta, StoryFn, type StoryObj } from '@storybook/react';
import styled from 'styled-components';

import { Flex, H2, Text } from 'design';

import { CollapsibleInfoSection as CollapsibleInfoSectionComponent } from './';

type Story = StoryObj<typeof CollapsibleInfoSectionComponent>;
type StoryMeta = Meta<typeof CollapsibleInfoSectionComponent>;

const defaultArgs = {
  size: 'large',
  defaultOpen: false,
  openLabel: 'More info',
  closeLabel: 'Less info',
  onClick: action('onClick'),
} satisfies StoryMeta['args'];

export default {
  title: 'Design/CollapsibleInfoSection',
  component: CollapsibleInfoSectionComponent,
  argTypes: {
    size: {
      control: { type: 'radio' },
      options: ['small', 'large'],
      description: 'Size of the toggle button',
      table: { defaultValue: { summary: defaultArgs.size } },
    },
    defaultOpen: {
      control: { type: 'boolean' },
      description: 'Whether the section is open or closed initially',
      table: {
        defaultValue: { summary: defaultArgs.defaultOpen.toString() },
      },
    },
    openLabel: {
      control: { type: 'text' },
      description: 'Label for the closed state',
      table: { defaultValue: { summary: defaultArgs.openLabel } },
    },
    closeLabel: {
      control: { type: 'text' },
      description: 'Label for the opened state',
      table: { defaultValue: { summary: defaultArgs.closeLabel } },
    },
    onClick: {
      control: false,
      description: 'Callback for when the toggle is clicked',
      table: { type: { summary: '(isOpen: boolean) => void' } },
    },
  },
  args: defaultArgs,
  render: (args => (
    <Container maxWidth="750px">
      <H2 ml={1}>Collapsible Info Section</H2>
      <Text ml={1} mb={1}>
        This is an example of a collapsible section that shows more information
        when expanded. It is useful for hiding less important – or more detailed
        – information by default.
      </Text>
      <CollapsibleInfoSectionComponent {...args} maxWidth="500px">
        <DummyContent />
      </CollapsibleInfoSectionComponent>
    </Container>
  )) satisfies StoryFn<typeof CollapsibleInfoSectionComponent>,
} satisfies StoryMeta;

const Container = styled(Flex).attrs({
  flexDirection: 'column',
  gap: 2,
})`
  background-color: ${p => p.theme.colors.spotBackground[0]};
  border-radius: ${p => `${p.theme.radii[3]}px`};
  padding: ${p => p.theme.space[3]}px;
`;

const DummyContent = () => (
  <Flex flexDirection="column" gap={2}>
    <Text bold>What this does:</Text>
    <Text>
      This is a dummy section that shows some information when the section is
      expanded. It can contain anything you want.
    </Text>
    <Text>
      There is no limit to the amount of content you can put in here, but it is
      recommended to keep it concise.
    </Text>
  </Flex>
);

export const CollapsibleInfoSection: Story = { args: defaultArgs };
