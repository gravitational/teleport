import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';
import Box from 'design/Box';
import * as Icons from 'design/Icon';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import Text from 'design/Text';

import cfg from 'teleport/config';
import { IntegrationKind } from 'teleport/services/integrations';
import localStorage from 'teleport/services/localStorage';
import useTeleport from 'teleport/useTeleport';

import { CtaEvent } from 'teleport/services/userEvent';

import { ButtonLockedFeature } from '../ButtonLockedFeature';

export const Container = () => {
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

  return (
    <ExternalAuditStorageCta
      showCta={showCta}
      onDismiss={handleDismiss}
      isEnabled={featureEnabled}
    />
  );
};

export type Props = {
  showCta: boolean;
  isEnabled: boolean;
  onDismiss: () => void;
};

export const ExternalAuditStorageCta = ({
  showCta,
  isEnabled,
  onDismiss,
}: Props) => {
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
                isEnabled ? '' : ' with Teleport Enterprise'
              }.`}
            </Text>
            <Link style={{ display: 'inline', marginLeft: 4 }} to={'TODO'}>
              {' '}
              Learn More
            </Link>
          </Box>
        </Flex>
        <Flex alignItems="center" minWidth="300px">
          {isEnabled ? (
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

          <ButtonSecondary onClick={onDismiss}>Dismiss</ButtonSecondary>
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
