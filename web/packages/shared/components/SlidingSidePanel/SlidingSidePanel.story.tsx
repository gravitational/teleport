/**
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

import { Meta } from '@storybook/react-vite';
import { useState } from 'react';

import { Box, ButtonPrimary } from 'design';
import { Info } from 'design/Alert';

import { InfoGuideContainer } from './InfoGuide';
import { LongGuideContent } from './InfoGuide/storyHelper';
import { SlidingSidePanel } from './SlidingSidePanel';

type StoryProps = {
  slideFrom: 'left' | 'right';
  panelOffset: string;
  skipAnimation: boolean;
  panelWidth: number;
};

const meta: Meta<StoryProps> = {
  title: 'Shared/SlidingSidePanel',
  component: Controls,
  argTypes: {
    slideFrom: {
      control: { type: 'select' },
      options: ['left', 'right'],
    },
    panelOffset: {
      control: { type: 'text' },
    },
    skipAnimation: {
      control: { type: 'boolean' },
    },
    panelWidth: {
      control: { type: 'number' },
    },
  },
  // default
  args: {
    slideFrom: 'right',
    panelOffset: '0',
    skipAnimation: false,
    panelWidth: 300,
  },
};
export default meta;

export function Controls(props: StoryProps) {
  const [show, setShow] = useState(false);

  return (
    <>
      <Box textAlign="center">
        <Info>
          Dev: The panel uses components defined in InfoGuide.tsx as examples
        </Info>
        <ButtonPrimary onClick={() => setShow(!show)}>Toggle Me</ButtonPrimary>
      </Box>
      <SlidingSidePanel
        isVisible={show}
        skipAnimation={props.skipAnimation}
        panelWidth={props.panelWidth}
        zIndex={1000}
        slideFrom={props.slideFrom}
        panelOffset={props.panelOffset}
      >
        <InfoGuideContainer
          onClose={() => setShow(false)}
          title="Some Guide Title"
        >
          <LongGuideContent />
        </InfoGuideContainer>
      </SlidingSidePanel>
    </>
  );
}
