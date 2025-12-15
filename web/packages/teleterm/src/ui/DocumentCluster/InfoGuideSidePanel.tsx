/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { SlidingSidePanel } from 'shared/components/SlidingSidePanel';
import {
  InfoGuideContainer,
  useInfoGuide,
} from 'shared/components/SlidingSidePanel/InfoGuide';

import { statusBarHeight } from '../StatusBar/constants';
import { tabHeight } from '../Tabs/constants';

/**
 * An info panel that always slides from the right and supports closing
 * from inside of panel (by clicking on x button from the sticky header).
 *
 * The panel will always render below teleterm's tabs and above
 * teleterm's status bar.
 */
export const InfoGuideSidePanel = () => {
  const { infoGuideConfig, setInfoGuideConfig, panelWidth } = useInfoGuide();
  const infoGuideSidePanelOpened = infoGuideConfig != null;

  return (
    <SlidingSidePanel
      isVisible={infoGuideSidePanelOpened}
      skipAnimation={false}
      panelWidth={panelWidth}
      zIndex={10}
      slideFrom="right"
      css={`
        top: ${p => p.theme.topBarHeight[1] + tabHeight}px;
        bottom: ${statusBarHeight}px;
        border-top: 1px solid
          ${p => p.theme.colors.interactive.tonal.neutral[0]};
      `}
    >
      <InfoGuideContainer
        onClose={() => setInfoGuideConfig(null)}
        title={infoGuideConfig?.title}
      >
        {infoGuideConfig?.guide}
      </InfoGuideContainer>
    </SlidingSidePanel>
  );
};
