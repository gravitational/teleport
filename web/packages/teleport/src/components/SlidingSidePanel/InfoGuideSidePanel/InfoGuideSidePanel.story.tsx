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

import { Box } from 'design';
import { Info } from 'design/Alert';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';
import { LongGuideContent } from 'shared/components/SlidingSidePanel/InfoGuide/storyHelper';

import { InfoGuideSidePanel as MainComponent } from './InfoGuideSidePanel';
import { TopBar } from './storyHelpers';

export default {
  title: 'Teleport/InfoGuideSidePanel',
};

export function InfoGuideSidePanel() {
  return (
    <TopBar>
      <Example />
    </TopBar>
  );
}

function Example() {
  return (
    <>
      <Box mt={10}>
        <Info>
          The top bar nav is only rendered for demo purposes (side panel should
          render below the nav)
        </Info>
        <InfoGuideButton config={{ guide: <LongGuideContent /> }}>
          <Box ml={3}>Click the info button</Box>
        </InfoGuideButton>
      </Box>
      <MainComponent />
    </>
  );
}
