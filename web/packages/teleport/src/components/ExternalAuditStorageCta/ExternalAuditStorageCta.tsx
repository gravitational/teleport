/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';
import Box from 'design/Box';
import * as Icons from 'design/Icon';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import Text from 'design/Text';

import localStorage from 'teleport/services/localStorage';
import cfg from 'teleport/config';
import { IntegrationKind } from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

import { CtaEvent } from 'teleport/services/userEvent';

import { ButtonLockedFeature } from '../ButtonLockedFeature';

export const ExternalAuditStorageCta = () => {
  const [showCta, setShowCta] = useState<boolean>(false);
  const ctx = useTeleport();
  const featureEnabled = !ctx.lockedFeatures.externalCloudAudit;

  useEffect(() => {
    setShowCta(
      cfg.isCloud && !localStorage.getExternalAuditStorageCtaDisabled()
    );
  }, [cfg.isCloud]);

  function handleDismiss() {
    localStorage.disableExternalAuditStorageCta();
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
        <Flex alignItems="center" minWidth="300px">
          {featureEnabled ? (
            <ButtonPrimary
              as={Link}
              to={cfg.getIntegrationEnrollRoute(
                IntegrationKind.ExternalAuditStorage
              )}
              mr="2"
            >
              Connect your AWS storage
            </ButtonPrimary>
          ) : (
            <ButtonLockedFeature
              height="32px"
              size="medium"
              event={CtaEvent.CTA_EXTERNAL_AUDIT_STORAGE}
              mr={5}
            >
              Contact Sales
            </ButtonLockedFeature>
          )}

          <ButtonSecondary onClick={handleDismiss}>Dismiss</ButtonSecondary>
        </Flex>
      </Flex>
    </CtaContainer>
  );
};

const CtaContainer = styled(Box)`
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border: 1px solid ${props => props.theme.colors.spotBackground[2]};
  border-radius: ${props => `${props.theme.space[2]}px`};
`;
