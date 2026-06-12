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

import {
  InfoGuideContainer,
  useInfoGuide,
} from 'shared/components/SlidingSidePanel/InfoGuide';

import { zIndexMap } from 'teleport/Navigation/zIndexMap';

import { SlidingSidePanel } from '../SlidingSidePanel';

/**
 * An info panel that always slides from the right and supports closing
 * from inside of panel (by clicking on x button from the sticky header).
 *
 * The panel will always render below the web ui's tob bar menu.
 */
export const InfoGuideSidePanel = () => {
  const { infoGuideConfig, setInfoGuideConfig, panelWidth } = useInfoGuide();
  const infoGuideSidePanelOpened = infoGuideConfig != null;

  return (
    <SlidingSidePanel
      isVisible={infoGuideSidePanelOpened}
      skipAnimation={false}
      panelWidth={panelWidth}
      zIndex={zIndexMap.infoGuideSidePanel}
      slideFrom="right"
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
