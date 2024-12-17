import { useState, useEffect } from 'react';
import { Prompt, useLocation } from 'react-router-dom';
import styled from 'styled-components';
import { Link } from 'react-router-dom';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Header } from 'teleport/Discover/Shared';
import {
  Text,
  Box,
  ButtonPrimary,
  Alert,
  Flex,
  ButtonSecondary,
  Image,
  H2,
} from 'design';
import { Notification as IconNotification } from 'design/Icon';

import celebratePamPng from 'teleport/Discover/Shared/Finished/celebrate-pam.png';

import cfg from 'teleport/config';

import useTeleport from 'teleport/useTeleport';

import useTeleportE from 'e-teleport/useTeleportE';

import { ConfigurePermissions } from './ConfigurePermissions';
import { TestConnection } from './TestConnection';

import { SelectIntegration } from './SelectIntegration';
import { Step, useExternalAuditStorage } from './useExternalAuditStorage';

export function ExternalAuditStorage() {
  const ctx = useTeleport();
  const { currentStep, setCurrentStep, draft, continuePreviousDraft } =
    useExternalAuditStorage();

  const { externalAuditStorageService } = useTeleportE();

  const { attempt: activateAttempt, run: activateRun } = useAttempt('');
  const [activated, setActivated] = useState<boolean>(false);

  const location = useLocation<{ continueDraft: boolean }>();
  useEffect(() => {
    // continuing draft (probably a redirect from integrations list)
    if (location.state?.continueDraft) {
      setCurrentStep(Step.TestConnection);
      continuePreviousDraft();
    }
  }, [location.state]);

  function activate() {
    activateRun(() =>
      externalAuditStorageService.promoteDraft().then(() => {
        setActivated(true);
        ctx.hasExternalAuditStorage = true;
      })
    );
  }

  // delete draft if the activation flow didn't
  // go all the way through
  const unfinishedFlow = !activated && draft !== null;
  useEffect(() => {
    let onBeforeUnload;
    let onUnload;

    // add a warning before closing the tab
    if (unfinishedFlow) {
      // ask for confirmation
      onBeforeUnload = (event: BeforeUnloadEvent) => {
        event.preventDefault();
      };

      // perform clean up
      onUnload = event => {
        event.preventDefault();
      };

      window.addEventListener('beforeunload', onBeforeUnload);
      window.addEventListener('unload', onUnload, true);
    }

    return () => {
      if (onBeforeUnload) {
        window.removeEventListener('beforeunload', onBeforeUnload);
      }
      if (onUnload) {
        window.removeEventListener('unload', onUnload);
      }
    };
  }, [draft, activated]);

  if (activated) {
    return (
      <Flex
        width="600px"
        flexDirection="column"
        alignItems="center"
        mt={5}
        css={`
          margin-right: auto;
          margin-left: auto;
          text-align: center;
        `}
      >
        <Image width="120px" height="120px" src={celebratePamPng} />
        <H2 mt={3} mb={2}>
          External Audit Storage Successfully Added
        </H2>
        <Text mb={3}>
          Audit data will now be stored in your infrastructure.
        </Text>
        <Flex>
          <ButtonPrimary
            mr="4"
            as={Link}
            to={cfg.routes.integrations}
            size="large"
          >
            Go to Integration List
          </ButtonPrimary>
          <ButtonSecondary
            as={Link}
            to={cfg.getIntegrationEnrollRoute(null)}
            size="large"
          >
            Add Another Integration
          </ButtonSecondary>
        </Flex>
      </Flex>
    );
  }
  return (
    <>
      <Box pt={3}>
        <Header>Configure External Audit Storage</Header>
        <Text>
          Teleport will create S3 buckets, a Glue database, and an Athena
          workgroup in your AWS account, and use these for future audit event
          and session storage and retrieval.{' '}
        </Text>
        <AlertContainer>
          <Notification mr="4" />
          <Box>
            <Text bold>
              Setting Up External Audit Storage Will Make Past Audit Events
              Inaccessible
            </Text>
            <Text>
              The External Audit Storage integration will store all new audit
              log events and session recordings on your own Amazon account, but
              any audit logs and session recordings from before the change will
              become inaccessible.
            </Text>
          </Box>
        </AlertContainer>
        <Box mt="4">
          <StepContainer mb="3">
            <Text bold>Step 1: Select Integration</Text>
            <SelectIntegration />
          </StepContainer>
          {currentStep >= Step.ConfigurePermissions && (
            <StepContainer mb="3">
              <Text bold>Step 2: Configure Permissions</Text>
              <ConfigurePermissions />
            </StepContainer>
          )}
          {currentStep >= Step.TestConnection && (
            <StepContainer mb="3">
              <Text bold>Step 3: Test Connection</Text>
              <TestConnection />
            </StepContainer>
          )}
          {currentStep >= Step.Activation && (
            <>
              {activateAttempt.status == 'failed' && (
                <Alert>
                  There was a problem activating the integration:{' '}
                  {activateAttempt.statusText}
                </Alert>
              )}

              <ButtonPrimary
                disabled={activateAttempt.status == 'processing'}
                onClick={activate}
                data-testid="activate-button"
              >
                Activate
              </ButtonPrimary>
            </>
          )}
        </Box>
      </Box>
      <Prompt message={PROMPT_MESSAGE} when={unfinishedFlow} />
    </>
  );
}

const StepContainer = styled(Box)`
  max-width: 1000px;
  background-color: ${p => p.theme.colors.spotBackground[0]};
  border-radius: ${p => `${p.theme.space[2]}px`};
  padding: ${p => p.theme.space[3]}px;
`;

const AlertContainer = styled(Box)`
  max-width: 1000px;
  margin: ${p => p.theme.space[4]}px 0;
  padding: ${p => p.theme.space[2]}px ${p => p.theme.space[3]}px;
  border: 2px solid #009eff;
  border-radius: 8px;
  background-color: #009eff1a;
  display: flex;
  align-items: center;
`;

const Notification = styled(IconNotification)`
  color: ${p => p.theme.colors.text.primaryInverse};
  padding: 8px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background-color: #009eff;
`;

const PROMPT_MESSAGE =
  'Are you sure you want to exit the "External Audit Storage" integration?\n\n' +
  'Any resources created by Teleport will not be automatically removed.\n\n' +
  'You can continue this integration later from the Integrations page.';
