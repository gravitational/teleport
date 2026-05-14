// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
