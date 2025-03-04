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

import { Meta } from '@storybook/react';
import { useState } from 'react';

import { Box, ButtonPrimary } from 'design';
import { Info } from 'design/Alert';

import { SlidingSidePanel } from './SlidingSidePanel';
import { LongContent, TopBar } from './storyHelpers';

type StoryProps = {
  slideFrom: 'left' | 'right';
  panelOffset: string;
  skipAnimation: boolean;
};

const meta: Meta<StoryProps> = {
  title: 'Teleport/SlidingSidePanel',
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
  },
  // default
  args: {
    slideFrom: 'right',
    panelOffset: '0',
    skipAnimation: false,
  },
};
export default meta;

export function Controls(props: StoryProps) {
  const [show, setShow] = useState(false);

  return (
    <TopBar>
      {/* this Box wrapper is just for demo purposes */}
      <Box mt={10} textAlign="center">
        <Info>The top bar nav is only rendered for demo purposes</Info>
        <ButtonPrimary onClick={() => setShow(!show)}>Toggle Me</ButtonPrimary>
      </Box>
      <SlidingSidePanel
        isVisible={show}
        skipAnimation={props.skipAnimation}
        panelWidth={300}
        zIndex={1000}
        slideFrom={props.slideFrom}
        panelOffset={props.panelOffset}
      >
        <LongContent />
      </SlidingSidePanel>
    </TopBar>
  );
}
