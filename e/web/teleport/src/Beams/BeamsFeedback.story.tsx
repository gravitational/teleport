import type { Meta, StoryObj } from '@storybook/react-vite';

import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { ContentMinWidth } from 'teleport/Main/Main';

import { BeamsFeedback } from './BeamsFeedback';

const meta = {
  title: 'TeleportE/Beams/BeamsFeedback',
  component: BeamsFeedback,
  decorators: [
    Story => (
      <InfoGuidePanelProvider>
        <ContentMinWidth>
          <Story />
        </ContentMinWidth>
      </InfoGuidePanelProvider>
    ),
  ],
} satisfies Meta<typeof BeamsFeedback>;

export default meta;

export const Default: StoryObj<typeof BeamsFeedback> = {};
