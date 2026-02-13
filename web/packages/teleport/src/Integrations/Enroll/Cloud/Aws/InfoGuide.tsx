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

import styled from 'styled-components';

import { Box, Flex, Text } from 'design';
import { Info } from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import { HoverTooltip } from 'design/Tooltip';
import {
  ViewModeSwitchButton,
  ViewModeSwitchContainer,
} from 'shared/components/Controls/ViewModeSwitch';
import {
  InfoParagraph,
  InfoTitle,
  ReferenceLinks,
  type ReferenceLink,
} from 'shared/components/SlidingSidePanel/InfoGuide';
import { marginTransitionCss } from 'shared/components/SlidingSidePanel/InfoGuide/const';
import { useValidation } from 'shared/components/Validation';

import LiveTextEditor from '../LiveTextEditor';
import { CopyTerraformButton } from './DeploymentMethodSection';

export const PANEL_WIDTH = 500;

export type InfoGuideTab = 'info' | 'terraform' | null;

export const ContentWithSidePanel = styled(Box)<{
  isPanelOpen: boolean;
  panelWidth: number;
  contentMinWidth?: number;
}>`
  min-width: ${props =>
    props.contentMinWidth ? `${props.contentMinWidth}px` : '650px'};

  ${props =>
    marginTransitionCss({
      sidePanelOpened: props.isPanelOpen,
      panelWidth: props.panelWidth,
    })}
`;

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
    <Flex
      ml={-3}
      width={`${PANEL_WIDTH - 2}px`}
      flexDirection="column"
      height="600px"
      position="sticky"
    >
      <LiveTextEditor
        data={[{ content: terraformConfig, type: 'terraform' }]}
      />
      <Box p={3}>
        <CopyTerraformButton
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

const referenceLinks: ReferenceLink[] = [
  {
    title: 'Teleport AWS Discovery Documentation',
    href: 'https://goteleport.com/docs/enroll-resources/auto-discovery/servers/ec2-discovery/',
  },
  {
    title: 'AWS IAM Roles',
    href: 'https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html',
  },
  {
    title: 'AWS Organizations',
    href: 'https://docs.aws.amazon.com/organizations/',
  },
];

export function InfoGuideContent() {
  return (
    <Box>
      <InfoTitle>Overview</InfoTitle>
      <InfoParagraph>
        Connect your AWS account to Teleport to automatically discover and
        enroll resources in your cluster.
      </InfoParagraph>

      <InfoTitle>How It Works</InfoTitle>
      <Box pl={2}>
        <ol
          css={`
            padding: inherit;
          `}
        >
          <li>
            <strong>Deploy IAM role with discovery permissions.</strong>
            <br /> Using Terraform, create an IAM role that grants Teleport
            read-only access to your AWS resources.
          </li>
          <li>
            <strong>Configure what to discover.</strong> <br />
            Specify regions, resource types (EC2, RDS, EKS), and tag filters to
            control which resources are discovered.
          </li>
          <li>
            <strong>Automatic discovery begins.</strong> <br />
            Teleport scans your AWS environment every 30 minutes to find
            resources matching your configuration.
          </li>
          <li>
            <strong>Resources appear in your cluster.</strong>
            <br /> Discovered resources are automatically enrolled in Teleport
            and ready for secure access.
          </li>
        </ol>
      </Box>
      <ReferenceLinks links={referenceLinks} />
    </Box>
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
