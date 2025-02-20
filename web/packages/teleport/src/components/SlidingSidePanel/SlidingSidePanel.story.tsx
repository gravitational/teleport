/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { useState } from 'react';

import { Box, ButtonPrimary, Text } from 'design';
import { Info } from 'design/Alert';

import { SlidingSidePanel } from './SlidingSidePanel';
import { LongContent, Providers } from './storyHelpers';

export default {
  title: 'Teleport/SlidingSidePanel',
};

const DevInfo = () => (
  <Info>The top bar nav is only rendered for demo purposes</Info>
);

export const SlideFromRight = () => {
  const [show, setShow] = useState(false);
  return (
    <Providers>
      {/* this Box wrapper is just for demo purposes */}
      <Box mt={10} textAlign="center">
        <DevInfo />
        <ButtonPrimary onClick={() => setShow(!show)}>Toggle Me</ButtonPrimary>
      </Box>
      <SlidingSidePanel
        isVisible={show}
        skipAnimation={false}
        panelWidth={300}
        zIndex={1000}
        slideFrom="right"
        right={0}
      >
        <LongContent />
      </SlidingSidePanel>
    </Providers>
  );
};

export const SlideFromLeft = () => {
  const [show, setShow] = useState(false);
  return (
    <Providers>
      {/* this Box wrapper is just for demo purposes */}
      <Box mt={10} ml={3} textAlign="center">
        <DevInfo />
        <ButtonPrimary onClick={() => setShow(!show)}>Toggle Me</ButtonPrimary>
      </Box>
      <SlidingSidePanel
        isVisible={show}
        skipAnimation={false}
        panelWidth={300}
        zIndex={1000}
        slideFrom="left"
        left={0}
      >
        <LongContent />
      </SlidingSidePanel>
    </Providers>
  );
};
