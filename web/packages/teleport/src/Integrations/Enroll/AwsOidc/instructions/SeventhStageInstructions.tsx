import React, { useState } from 'react';
import { useLocation } from 'react-router';
import { Link } from 'react-router-dom';
import { Danger } from 'design/Alert';
import { CircleCheck } from 'design/Icon';
import { ButtonPrimary, ButtonSecondary, Text, Flex, Box } from 'design';
import Dialog, {
  DialogHeader,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import useAttempt from 'shared/hooks/useAttemptNext';

import { requiredField } from 'shared/components/Validation/rules';

import {
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import cfg from 'teleport/config';

import { InstructionsContainer } from './common';

export function SeventhStageInstructions() {
  const location = useLocation<{ discoverEventId: string }>();

  const { attempt, setAttempt } = useAttempt('');
  const [showConfirmBox, setShowConfirmBox] = useState(false);
  const [roleArn, setRoleArn] = useState('');
  const [name, setName] = useState('');

  function handleSubmit(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    setAttempt({ status: 'processing' });
    integrationService
      .createIntegration({
        name,
        subKind: IntegrationKind.AwsOidc,
        awsoidc: { roleArn },
      })
      .then(() => setShowConfirmBox(true))
      .catch((err: Error) =>
        setAttempt({ status: 'failed', statusText: err.message })
      );
  }

  return (
    <InstructionsContainer>
      {attempt.status === 'failed' && (
        <Danger mb={5}>{attempt.statusText}</Danger>
      )}
      <Text>From the list of roles, select the role you just created</Text>

      <Validation>
        {({ validator }) => (
          <>
            <Text mt={5}>Copy the role ARN and paste it below</Text>
            <Box mt={2}>
              <FieldInput
                label="Role ARN"
                onChange={e => setRoleArn(e.target.value)}
                value={roleArn}
                placeholder="Role ARN"
                rule={requiredField('Role ARN is required')}
              />
            </Box>
            <Text mt={5}>Give this AWS integration a name</Text>
            <Box mt={2}>
              <FieldInput
                label="Name"
                onChange={e => setName(e.target.value)}
                value={name}
                placeholder="Integration name"
                rule={requiredField('Name is required')}
              />
            </Box>
            <Box mt={5}>
              <ButtonPrimary
                onClick={() => handleSubmit(validator)}
                disabled={attempt.status === 'processing'}
              >
                Next
              </ButtonPrimary>
            </Box>
          </>
        )}
      </Validation>
      {showConfirmBox && (
        <SuccessfullyAddedIntegrationDialog
          discoverEventId={location.state?.discoverEventId}
          integrationName={name}
        />
      )}
    </InstructionsContainer>
  );
}

export function SuccessfullyAddedIntegrationDialog({
  discoverEventId,
  integrationName,
}: {
  discoverEventId: string;
  integrationName: string;
}) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={true}
      onClose={close}
      open={true}
    >
      <DialogHeader css={{ margin: '0 auto' }}>
        <CircleCheck mb={4} fontSize={60} color="success" />
      </DialogHeader>
      <DialogContent>
        <Text textAlign="center">
          AWS integration "{integrationName}" successfully added
        </Text>
      </DialogContent>
      <DialogFooter css={{ margin: '0 auto' }}>
        {discoverEventId ? (
          <ButtonPrimary
            size="large"
            as={Link}
            to={{
              pathname: cfg.routes.discover,
              state: {
                integrationSubKind: IntegrationKind.AwsOidc,
                integrationName,
                discoverEventId,
              },
            }}
          >
            Begin RDS Enrollment
          </ButtonPrimary>
        ) : (
          <Flex gap="3">
            <ButtonPrimary as={Link} to={cfg.routes.integrations} size="large">
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
        )}
      </DialogFooter>
    </Dialog>
  );
}
