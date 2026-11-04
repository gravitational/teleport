/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Box, H2, ResourceIcon, Text } from 'design';
import { Info } from 'design/Icon';

import {
  ResizableSidePanelMainContent,
  getPanelTitle,
  ResizablePanelLayout,
  ResizableSidePanel,
  useResizableSidePanel,
} from '.';
import { SidePanelTab, SidePanelTabs } from './SidePanelTab';

export default {
  title: 'Teleport/ResizableSidePanel',
};

export function Example() {
  const panel = useResizableSidePanel();

  return (
    <Box height="600px">
      <ResizablePanelLayout $withHorizontalScroll>
        <ResizableSidePanelMainContent $minWidth={650}>
          <Box p={4}>
            <H2 mb={3}>Resizable Side Panel</H2>
            <Text mb={2}>
              Drag the panel&apos;s left handle/edge to resize it between its
              min/max widths.
            </Text>
            <Text>
              Switch tabs, close the panel, and reopen it to see that the
              selected width is preserved.
            </Text>
          </Box>
        </ResizableSidePanelMainContent>

        <SidePanelTabs>
          <SidePanelTab
            ariaLabel={getPanelTitle('info')}
            isActive={panel.isOpen && panel.activeTab === 'info'}
            onClick={() => panel.toggleTab('info')}
          >
            <Info size="small" />
          </SidePanelTab>

          <SidePanelTab
            ariaLabel={getPanelTitle('terraform')}
            isActive={panel.isOpen && panel.activeTab === 'terraform'}
            onClick={() => panel.toggleTab('terraform')}
          >
            <ResourceIcon name="terraform" width="20px" height="20px" />
          </SidePanelTab>
        </SidePanelTabs>

        {panel.isOpen && (
          <ResizableSidePanel
            title={getPanelTitle(panel.activeTab)}
            width={panel.width}
            onWidthChange={panel.updateWidth}
            onClose={panel.close}
          >
            <Box p={3}>
              {panel.activeTab === 'info' ? (
                <>
                  <H2 mb={2}>Information</H2>
                  <Text>
                    This panel starts open and can be resized, closed, and
                    reopened at its previously selected width.
                  </Text>
                </>
              ) : (
                <Text>some terraform configuration content</Text>
              )}
            </Box>
          </ResizableSidePanel>
        )}
      </ResizablePanelLayout>
    </Box>
  );
}
