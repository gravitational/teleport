import type { Meta, StoryObj } from '@storybook/react-vite';
import { MemoryRouter } from 'react-router';

import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { ContentMinWidth } from 'teleport/Main/Main';

import { BeamsList } from './BeamsList';

const meta = {
  title: 'TeleportE/Beams/BeamsList',
  component: BeamsList,
  decorators: [
    Story => (
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContentMinWidth>
            <Story />
          </ContentMinWidth>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    ),
  ],
} satisfies Meta<typeof BeamsList>;

export default meta;

export const Default: StoryObj<typeof BeamsList> = {};
