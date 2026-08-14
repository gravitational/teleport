import { Meta, StoryObj } from '@storybook/react-vite';

import { Flex } from 'design';

import { ContentMinWidth } from 'teleport/Main/Main';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';

import { BeamsQuickstart } from '.';

const meta = {
  title: 'Teleport/Beams',
  component: Wrapper,
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

export const Quickstart: Story = {};

function Wrapper() {
  const ctx = createTeleportContext();
  return (
    <TeleportProviderBasic teleportCtx={ctx}>
      <Flex flexDirection="column" height="100vh">
        <ContentMinWidth>
          <BeamsQuickstart />
        </ContentMinWidth>
      </Flex>
    </TeleportProviderBasic>
  );
}
