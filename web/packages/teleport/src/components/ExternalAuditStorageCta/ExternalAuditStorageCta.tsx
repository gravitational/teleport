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

import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';
import Box from 'design/Box';
import * as Icons from 'design/Icon';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import Text from 'design/Text';

import { HoverTooltip } from 'shared/components/ToolTip';

import cfg from 'teleport/config';
import { IntegrationKind } from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

import { CtaEvent } from 'teleport/services/userEvent';

import { storageService } from 'teleport/services/storageService';

import { ButtonLockedFeature } from '../ButtonLockedFeature';

export const ExternalAuditStorageCta = () => {
  const [showCta, setShowCta] = useState<boolean>(false);
  const ctx = useTeleport();
  const featureEnabled = !cfg.externalAuditStorage;
  const userHasAccess = ctx.getFeatureFlags().enrollIntegrationsOrPlugins;

  useEffect(() => {
    setShowCta(
      !ctx.hasExternalAuditStorage &&
        cfg.isCloud &&
        !storageService.getExternalAuditStorageCtaDisabled()
    );
  }, [cfg.isCloud]);

  function handleDismiss() {
    storageService.disableExternalAuditStorageCta();
    setShowCta(false);
  }

  if (!showCta) {
    return null;
  }

  return (
    <CtaContainer mb="4">
      <Flex justifyContent="space-between">
        <Flex mr="4" alignItems="center">
          <Icons.Server size="medium" mr="3" />
          <Box>
            <Text bold>External Audit Storage</Text>
            <Text style={{ display: 'inline' }}>
              {`Connect your own AWS account to store audit logs and session recordings on your own infrastructure${
                featureEnabled ? '' : ' with Teleport Enterprise'
              }.`}
            </Text>
          </Box>
        </Flex>
        <Flex alignItems="center" minWidth="300px" gap={2}>
          <CtaButton
            featureEnabled={featureEnabled}
            userHasAccess={userHasAccess}
          />
          <ButtonSecondary onClick={handleDismiss}>Dismiss</ButtonSecondary>
        </Flex>
      </Flex>
    </CtaContainer>
  );
};

function CtaButton(props: { featureEnabled: boolean; userHasAccess: boolean }) {
  if (!props.featureEnabled) {
    return (
      <ButtonLockedFeature
        height="32px"
        size="medium"
        event={CtaEvent.CTA_EXTERNAL_AUDIT_STORAGE}
      >
        Contact Sales
      </ButtonLockedFeature>
    );
  }

  return (
    <HoverTooltip
      tipContent={
        props.userHasAccess
          ? ''
          : `Insufficient permissions. Reach out to your Teleport administrator
    to request External Audit Storage permissions.`
      }
    >
      <ButtonPrimary
        as={Link}
        to={cfg.getIntegrationEnrollRoute(IntegrationKind.ExternalAuditStorage)}
        disabled={!props.userHasAccess}
      >
        Connect your AWS storage
      </ButtonPrimary>
    </HoverTooltip>
  );
}

const CtaContainer = styled(Box)`
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border: 1px solid ${props => props.theme.colors.spotBackground[2]};
  border-radius: ${props => `${props.theme.space[2]}px`};
`;
