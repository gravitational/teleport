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

import { ReactNode, useState } from 'react';
import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import { Info } from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import { HoverTooltip } from 'design/Tooltip';
import {
  ViewModeSwitchButton,
  ViewModeSwitchContainer,
} from 'shared/components/Controls/ViewModeSwitch';
import { InfoGuideContainer } from 'shared/components/SlidingSidePanel/InfoGuide';
import { useValidation } from 'shared/components/Validation';

import { SlidingSidePanel } from 'teleport/components/SlidingSidePanel/SlidingSidePanel';
import { TerraformCopyButton } from 'teleport/components/TerraformCopyButton';
import { zIndexMap } from 'teleport/Navigation/zIndexMap';

import LiveTextEditor from './LiveTextEditor';

const responsivePanelWidth =
  'clamp(500px, calc(100vw - var(--sidenav-width, 84px) - 800px), 700px)';

export type InfoGuideTab = 'info' | 'terraform' | null;

export const ContentWithSidePanel = styled(Box)<{
  isPanelOpen: boolean;
}>`
  min-width: 650px;
  margin-right: ${p => (p.isPanelOpen ? responsivePanelWidth : '0')};
  transition: ${p => (p.isPanelOpen ? 'margin 150ms' : 'margin 300ms')};
`;

export function useTerraformInfoGuide(defaultTab: InfoGuideTab = 'info') {
  const [activeInfoGuideTab, setActiveInfoGuideTab] =
    useState<InfoGuideTab>(defaultTab);

  const isPanelOpen = activeInfoGuideTab !== null;

  const onInfoGuideClick = (section: InfoGuideTab) => {
    if (activeInfoGuideTab === section) {
      setActiveInfoGuideTab(null);
    } else {
      setActiveInfoGuideTab(section);
    }
  };

  return {
    isPanelOpen,
    activeInfoGuideTab,
    setActiveInfoGuideTab,
    onInfoGuideClick,
  };
}

export type TerraformInfoGuideProps = {
  terraformConfig: string;
  handleCopy: () => void;
};

export function TerraformInfoGuide({
  terraformConfig,
  handleCopy,
}: TerraformInfoGuideProps) {
  const validator = useValidation();

  return (
    <Flex mx={-3} flexDirection="column" height="600px" position="sticky">
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

export type InfoGuideTitleProps = {
  activeSection: 'info' | 'terraform';
  onSectionChange: (section: 'info' | 'terraform') => void;
};

export function InfoGuideTitle({
  activeSection,
  onSectionChange,
}: InfoGuideTitleProps) {
  return (
    <Flex alignItems="center" gap={3}>
      <InfoGuideTab
        active={activeSection === 'info'}
        onClick={() => onSectionChange('info')}
      >
        Info Guide
      </InfoGuideTab>
      <InfoGuideTab
        active={activeSection === 'terraform'}
        onClick={() => onSectionChange('terraform')}
      >
        Terraform
      </InfoGuideTab>
    </Flex>
  );
}

export const InfoGuideTab = styled(Text)<{ active: boolean }>`
  cursor: pointer;
  padding: 4px 8px;
  border-bottom: 2px solid
    ${p =>
      p.active
        ? p.theme.colors.interactive.solid.primary.default
        : 'transparent'};
  color: ${p =>
    p.active ? p.theme.colors.interactive.solid.primary.default : 'inherit'};
`;

type InfoGuideSwitchProps = {
  activeTab: InfoGuideTab;
  isPanelOpen: boolean;
  onSwitch: (activeTab: InfoGuideTab) => void;
};

export const InfoGuideSwitch = ({
  activeTab,
  isPanelOpen,
  onSwitch,
}: InfoGuideSwitchProps) => {
  return (
    <ViewModeSwitchContainer
      aria-label="Info Guide Mode Switch"
      aria-orientation="horizontal"
      role="radiogroup"
    >
      <HoverTooltip tipContent="Info Guide">
        <ViewModeSwitchButton
          className={isPanelOpen && activeTab === 'info' ? 'selected' : ''}
          onClick={() => onSwitch('info')}
          role="radio"
          aria-checked={isPanelOpen && activeTab === 'info'}
          aria-label="Info Guide"
          first
        >
          <Info size="small" color="text.main" />
        </ViewModeSwitchButton>
      </HoverTooltip>
      <HoverTooltip tipContent="Terraform Configuration">
        <ViewModeSwitchButton
          className={isPanelOpen && activeTab === 'terraform' ? 'selected' : ''}
          onClick={() => onSwitch('terraform')}
          role="radio"
          aria-checked={isPanelOpen && activeTab === 'terraform'}
          aria-label="Terraform Configuration"
          last
        >
          <ResourceIcon name="terraform" width="16px" height="16px" />
        </ViewModeSwitchButton>
      </HoverTooltip>
    </ViewModeSwitchContainer>
  );
};

type TerraformInfoGuideSidePanelProps = {
  activeTab: InfoGuideTab;
  onTabChange: (tab: InfoGuideTab) => void;
  InfoGuideContent: ReactNode;
  TerraformContent: ReactNode;
};

export function TerraformInfoGuideSidePanel({
  activeTab,
  onTabChange,
  InfoGuideContent,
  TerraformContent,
}: TerraformInfoGuideSidePanelProps) {
  return (
    <SlidingSidePanel
      css={`
        && {
          width: ${responsivePanelWidth};
        }
      `}
      isVisible={activeTab !== null}
      skipAnimation={false}
      panelWidth={0}
      zIndex={zIndexMap.infoGuideSidePanel}
      slideFrom="right"
    >
      <InfoGuideContainer
        onClose={() => onTabChange(null)}
        title={
          <InfoGuideTitle
            activeSection={activeTab}
            onSectionChange={onTabChange}
          />
        }
      >
        {activeTab === 'terraform' ? TerraformContent : InfoGuideContent}
      </InfoGuideContainer>
    </SlidingSidePanel>
  );
}
