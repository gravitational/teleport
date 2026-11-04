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

import { ReactNode, useEffect, useState } from 'react';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import { Info } from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import { useValidation } from 'shared/components/Validation';

import {
  getPanelTitle,
  ResizableSidePanel,
  useResizableSidePanel,
} from 'teleport/components/SlidingSidePanel/ResizableSidePanel';
import {
  SidePanelTab,
  SidePanelTabs,
} from 'teleport/components/SlidingSidePanel/ResizableSidePanel/SidePanelTab';
import { TerraformCopyButton } from 'teleport/components/TerraformCopyButton';

import LiveTextEditor from './LiveTextEditor';

type TerraformInfoGuideProps = {
  terraformConfig: string;
  handleCopy: () => void;
};

function TerraformInfoGuide({
  terraformConfig,
  handleCopy,
}: TerraformInfoGuideProps) {
  const validator = useValidation();

  return (
    <Flex
      mx={-3}
      flexDirection="column"
      position="sticky"
      height={'100%'}
      flex={'1'}
      minHeight={0}
    >
      <LiveTextEditor
        data={[{ content: terraformConfig, type: 'terraform' }]}
      />
      <Box p={3}>
        <TerraformCopyButton
          onClick={e => {
            const isValid = validator.validate();
            if (!isValid) {
              e.preventDefault();
            } else {
              handleCopy();
            }
          }}
        />
        {validator.state.validating && !validator.state.valid && (
          <Text color="error.main" mt={2} fontSize={1}>
            Please complete the required fields
          </Text>
        )}
      </Box>
    </Flex>
  );
}

export function ResizableTerraformInfoGuideSidePanel({
  infoGuideContent,
  terraformConfig,
  handleCopy,
}: {
  infoGuideContent: ReactNode;
  terraformConfig: string;
  handleCopy(): void;
}) {
  const sidePanel = useResizableSidePanel();
  const isTerraformPanelActive =
    sidePanel.isOpen && sidePanel.activeTab === 'terraform';

  const [lastSeenTerraformConfig, setLastSeenTerraformConfig] =
    useState(terraformConfig);

  useEffect(() => {
    if (isTerraformPanelActive) {
      setLastSeenTerraformConfig(terraformConfig);
    }
  }, [isTerraformPanelActive, terraformConfig]);

  const hasUnseenTerraformChanges =
    !isTerraformPanelActive && terraformConfig !== lastSeenTerraformConfig;

  return (
    <>
      <SidePanelTabs>
        <SidePanelTab
          ariaLabel="Info Guide"
          isActive={sidePanel.isOpen && sidePanel.activeTab === 'info'}
          onClick={() => sidePanel.toggleTab('info')}
        >
          <Info size="small" />
        </SidePanelTab>

        <SidePanelTab
          ariaLabel="Terraform Configuration"
          isActive={sidePanel.isOpen && sidePanel.activeTab === 'terraform'}
          onClick={() => sidePanel.toggleTab('terraform')}
        >
          <Box position="relative">
            <ResourceIcon name="terraform" width="20px" height="20px" />
            {hasUnseenTerraformChanges && <NotificationDot />}
          </Box>
        </SidePanelTab>
      </SidePanelTabs>
      {sidePanel.isOpen && (
        <ResizableSidePanel
          title={getPanelTitle(sidePanel.activeTab)}
          width={sidePanel.width}
          onWidthChange={sidePanel.updateWidth}
          onClose={sidePanel.close}
        >
          {sidePanel.activeTab === 'info' ? (
            <Box px={3} pb={3}>
              {infoGuideContent}
            </Box>
          ) : (
            <TerraformPanelContent px={3} pb={3}>
              <TerraformInfoGuide
                terraformConfig={terraformConfig}
                handleCopy={handleCopy}
              />
            </TerraformPanelContent>
          )}
        </ResizableSidePanel>
      )}
    </>
  );
}

const TerraformPanelContent = styled(Flex)`
  flex: 1;
  flex-direction: column;
  min-height: 0;
`;

const NotificationDot = styled.span`
  position: absolute;
  width: 8px;
  height: 8px;
  top: -4px;
  right: -6px;
  border-radius: 50%;
  background-color: ${p => p.theme.colors.interactive.solid.accent.default};
`;
