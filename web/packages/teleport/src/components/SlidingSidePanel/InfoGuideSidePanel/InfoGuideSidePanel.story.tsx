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

import { Box } from 'design';

import { useInfoGuide } from 'teleport/Main/InfoGuideContext';

import { DevInfo, LongContent, TopBar } from '../storyHelpers';
import {
  InfoGuideSidePanel as Component,
  InfoGuideWrapper,
} from './InfoGuideSidePanel';

export default {
  title: 'Teleport/SlidingSidePanel',
};

export const InfoGuideSidePanel = () => {
  return (
    <TopBar>
      {/* this Box wrapper is just for demo purposes */}
      <Box mt={10} ml={3}>
        <DevInfo />
        <InfoGuideWrapper guide={<LongContent />}>
          Click on the info icon
        </InfoGuideWrapper>
      </Box>
      <SidePanel />
    </TopBar>
  );
};

const SidePanel = () => {
  const { infoGuideElement, setInfoGuideElement } = useInfoGuide();

  return (
    <Component
      isVisible={infoGuideElement != null}
      onClose={() => setInfoGuideElement(null)}
    >
      <LongContent />
    </Component>
  );
};
