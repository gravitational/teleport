import { VStack } from '@chakra-ui/react';
import type { Meta, StoryObj } from '@storybook/react';

import { Input } from './Input';

const meta: Meta<typeof Input> = {
  component: Input,
  title: 'Chakra UI/Input',
};

export default meta;

type Story = StoryObj<typeof Input>;

export const Individual: Story = {
  name: 'Individual Component',
};

export const Variations: Story = {
  name: 'Variations',
  render: () => (
    <VStack>
      <Input placeholder="Enter SomeText" />

      <Input placeholder="Enter SomeText" hasError={true} />
    </VStack>
  ),
};
